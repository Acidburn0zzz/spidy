package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ardanlabs/kit/cfg"
	"github.com/ardanlabs/kit/log"
	"github.com/ardanlabs/spidy/spidy"
)

//==============================================================================

const context = "Spidy"

var events Events

// Events defines a event logger.
type Events struct{}

// ErrorEvent logs error message for events.
func (Events) ErrorEvent(context interface{}, event string, err error, format string, data ...interface{}) {
	// fmt.Printf("Error: %s : %s : %s : %s\n", context, event, err, fmt.Sprintf(format, data...))
	log.Error(context, event, err, format, data...)
}

// Event logs standard message for events.
func (Events) Event(context interface{}, event string, format string, data ...interface{}) {
	// fmt.Printf("Event: %s : %s : %s\n", context, event, fmt.Sprintf(format, data...))
	log.Dev(context, event, format, data...)
}

//==============================================================================

func main() {
	log.Init(os.Stdout, func() int { return log.DEV }, log.Ldefault)

	var link = flag.String("url", "", "Target URL for crawling")
	var workerCount = flag.Int("workers", 100, "Maximum workers to use in crawling")
	var timeout = flag.Int("timeout", 10000, "Maximum timeout before HEAD requests fails in milliseconds")
	var doExternals = flag.Bool("externals", false, "flag to crawl none external links. Defaults to true")

	flag.Parse()

	flag.Usage = func() {
		fmt.Println(`
Spidy - A simple deadlink finder.

Flags:

 -w "Maximum workers to be used for crawling pages, defaults to 100"
 -url "URL to crawl for dead links"
 -hostOnly "A boolean flag which allows setting whether external links should be considered"

Usage:

	// To crawl the giving url and no external links as well
	spidy -url http://golang.org
	spidy -url http://golang.org -externals true

	// To crawl the giving url and external links as well
	spidy -url http://golang.org -externals false

	// To crawl the giving url and set maximum possible workers and a custom timeout
	// for HEAD requests in milliseconds
	spidy -url http://golang.org -workers 300 -timeout 300

`)
	}

	var target string
	var allPaths bool

	workers := 100
	httpTimeout := 10000

	if err := cfg.Init(cfg.EnvProvider{Namespace: "SPIDY"}); err == nil {

		cfg.MustString("TARGET_URL")

		if tm, err := cfg.Int("HTTP_TIMEOUT"); err != nil {
			httpTimeout = tm
		}

		if ap, err := cfg.Bool("EXTERNAL_LINKS"); err != nil {
			allPaths = ap
		}

		if wo, err := cfg.Int("MAX_WORKERS"); err != nil {
			workers = wo
		}

	} else {

		if *link == "" {
			events.ErrorEvent(context, "main", err, "Configuration Error : Initialization Failed")
			os.Exit(1)
		}

		target = *link
		workers = *workerCount
		allPaths = *doExternals
		httpTimeout = *timeout
	}

	start := time.Now()
	ms := time.Duration(httpTimeout) * time.Millisecond

	conf := spidy.Config{
		Client:  &http.Client{Timeout: ms},
		URL:     target,
		All:     allPaths,
		Workers: workers,
		Depth:   -1,
		Events:  events,
	}

	deadlinks, err := spidy.Run(context, &conf)
	if err != nil {
		events.ErrorEvent(context, "main", err, "Completed")
		os.Exit(1)
	}

	end := time.Now().UTC()

	fmt.Println("--------------------Timelapse-------------------------------")
	fmt.Printf(`
Start Time: %s
End Time: %s
Duration: %s
`, start, end, end.Sub(start))

	if len(deadlinks) > 0 {
		fmt.Println("--------------------DEAD LINKS------------------------------")

		for _, f := range deadlinks {
			fmt.Printf(`
URL: %s
Status Code: %d
Error: %s

`, f.Link, f.Status, f.Error)
		}
		fmt.Println("------------------------------------------------------------")

		os.Exit(-1)
	}

	fmt.Println("------------------------------------------------------------")
}
