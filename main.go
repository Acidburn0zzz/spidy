package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ardanlabs/kit/cfg"
	"github.com/ardanlabs/spidy/spidy"
)

//==============================================================================

const context = "Spidy"

var events Events

// Events defines a event logger.
type Events struct{}

// ErrorEvent logs error message for events.
func (Events) ErrorEvent(context interface{}, event string, err error, format string, data ...interface{}) {
	fmt.Printf("Error: %s : %s : %s : %s\n", context, event, err, fmt.Sprintf(format, data...))
}

// Event logs standard message for events.
func (Events) Event(context interface{}, event string, format string, data ...interface{}) {
	fmt.Printf("Event: %s : %s : %s\n", context, event, fmt.Sprintf(format, data...))
}

//==============================================================================

func main() {

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
	spidy -url http://golang.org -hostOnly true

	// To crawl the giving url and external links as well
	spidy -url http://golang.org -hostOnly false

	// To crawl the giving url and set maximum possible workers
	spidy -url http://golang.org -w 300

`)
	}

	if err := cfg.Init(cfg.EnvProvider{Namespace: "SPIDY"}); err != nil {
		events.ErrorEvent(context, "main", err, "Configuration Error : Initialization Failed")
		os.Exit(1)
	}

	target := cfg.MustString("TARGET_URL")
	hostOnly := cfg.MustBool("HOST_ONLY")

	workers, err := cfg.Int("MAX_WORKERS")
	if err != nil {
		workers = 100
	}

	start := time.Now()

	deadlinks, err := spidy.Run(context, target, hostOnly, workers, events)
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

`, f.Link, f.Status)
		}
		fmt.Println("------------------------------------------------------------")

		os.Exit(-1)
	}

	fmt.Println("------------------------------------------------------------")
}
