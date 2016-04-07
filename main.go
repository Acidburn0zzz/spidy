package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ardanlabs/kit/pool"
)

func event(context interface{}, event string, format string, data ...interface{}) {
	fmt.Printf("Event: %s : %s : %s\n", context, event, fmt.Sprintf(format, data...))
}

// httpClient to provide http requests abilities.
var httpClient = http.Client{
	Timeout: 30 * time.Second,
}

type linkReport struct {
	Link   string
	Status int
	Error  error
}

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

	var workers = flag.Int("w", 100, "Url to crawl for dead links")
	var link = flag.String("url", "", "Maximum workers to be used for crawling pages, defaults to 100")
	var hostOnly = flag.Bool("hostOnly", true, "flag to crawl none base host. Defaults to true")

	flag.Parse()

	fmt.Printf("Crawler::URL[%s]\n", *link)
	fmt.Printf("Crawler::Crawl External Links[%t]\n", !*hostOnly)
	fmt.Printf("Crawler::Workers[%d]\n", *workers)

	if *workers <= 0 {
		*workers = 100
	}

	path, err := url.Parse(*link)
	if err != nil {
		fmt.Printf("Error: Unparsable URI: %s\n", *link)
		os.Exit(-1)
		return
	}

	// deadLinks provides a lists to contain all failed url links that are
	// considered deadlinks.
	var deadLinks []linkReport

	start := time.Now().UTC()

	dead := make(chan *linkReport)

	go collectFrom(path, !*hostOnly, *workers, dead)

	for {
		select {
		case <-time.After(time.Second):
			continue

		case link, ok := <-dead:
			if !ok {
				end := time.Now().UTC()
				fmt.Println("--------------------Timelapse-------------------------------")
				fmt.Printf(`
Start Time: %s
End Time: %s
Duration: %s
`, start, end, end.Sub(start))

				if len(deadLinks) > 0 {
					fmt.Println("--------------------DEAD LINKS------------------------------")
					for _, f := range deadLinks {
						fmt.Printf(`
URL: %s
Status Code: %d

`, f.Link, f.Status)
					}
					os.Exit(-1)
					return
				}

				fmt.Println("------------------------------------------------------------")

				return
			}

			deadLinks = append(deadLinks, *link)
		}
	}

}

// collectFrom uses a recursive function to map out the needed lists of links to.
// It returns a channel through which the acceptable links can be crawled from.
func collectFrom(path *url.URL, doExternals bool, maxWorkers int, dead chan *linkReport) {
	poolCfg := pool.Config{
		OptEvent:    pool.OptEvent{Event: event},
		MinRoutines: func() int { return 30 },
		MaxRoutines: func() int { return maxWorkers },
	}

	// create a new pool.
	pl, err := pool.New("spidy", "collectFrom", poolCfg)
	if err != nil {
		fmt.Printf("Spidy failed to create work pool: %s\n", err.Error())
		close(dead)
		return
	}

	status, crawleable, err := evaluatePath(path.String())
	if err != nil {
		pl.Shutdown("spidy")
		dead <- &linkReport{Link: path.String(), Status: status, Error: err}
		close(dead)
		return
	}

	defer pl.Shutdown("spidy")
	defer close(dead)

	if !crawleable {
		return
	}

	var wait sync.WaitGroup
	wait.Add(1)

	// vl provides a rwmutex for control concurrent reads and writes on the visited
	// map.
	var vl sync.RWMutex

	// visited is a map for storing visited uri's to avoid visit loops.
	visited := make(map[string]bool)
	visited[path.String()] = true

	go pl.Do("collectFrom", &pathBot{
		path:      path.String(),
		index:     path,
		dead:      dead,
		wait:      &wait,
		vl:        &vl,
		visited:   visited,
		pool:      pl,
		externals: doExternals,
		skipCheck: true,
	})

	wait.Wait()

	return
}

//==============================================================================

// pathBot provides a worker which checks a giving URL path, cascading its
// effects down its subroots and rescheduling new workers for those sublinks.
// It implements pool.Work interface.
type pathBot struct {
	path      string
	dead      chan *linkReport
	wait      *sync.WaitGroup
	vl        *sync.RWMutex
	visited   map[string]bool
	pool      *pool.Pool
	index     *url.URL
	skipCheck bool
	externals bool
}

// Work performs the necessary tasks of validating a link and rescheduling
// checks for sublinks.
func (p *pathBot) Work(context interface{}, id int) {
	// defer fmt.Printf("Ending task %d -> %s\n", id, p.path)
	defer p.wait.Done()

	p.vl.Lock()
	p.visited[p.path] = true
	p.vl.Unlock()

	if !p.skipCheck {
		status, crawleable, err := evaluatePath(p.path)
		if err != nil {
			p.dead <- &linkReport{Link: p.path, Status: status, Error: err}
			return
		}

		if !crawleable {
			return
		}
	}

	links := make(chan string)

	if err := farmLinks(p.path, links); err != nil {
		fmt.Printf("Spidy Failed to Farm Links for Page[%s]: Error[%s]\n", p.path, err.Error())
		p.dead <- &linkReport{Link: p.path, Status: http.StatusInternalServerError, Error: err}
		return
	}

	for {
		select {
		case <-time.After(time.Second):
			continue
		case link, ok := <-links:
			if !ok {
				return
			}

			p.vl.RLock()
			found := p.visited[link]
			p.vl.RUnlock()

			if found {
				continue
			}

			// If its a hash based path, then just add it as seen and skip.
			if strings.HasPrefix(link, "#") {
				p.vl.Lock()
				p.visited[link] = true
				p.vl.Unlock()
				continue
			}

			// If we are running into the same root link, skip it.
			if strings.TrimSpace(link) == "/" || link == p.index.Path {
				continue
			}

			pathURI, err := parsePath(link, p.index)
			if err != nil {
				continue
			}

			// If we are are not allowed external links, then check and if not
			// within host then skip but add it to scene list, we dont, want
			// to go through the same link twice.
			if !p.externals && !strings.Contains(pathURI.Host, p.index.Host) {
				p.vl.Lock()
				p.visited[link] = true
				p.vl.Unlock()
				continue
			}

			// Although we have fixed the path, we still need to add it down into the list
			// of visited.
			p.vl.Lock()
			p.visited[link] = true
			p.vl.Unlock()

			p.wait.Add(1)

			// collect(pathURI.String(), host, doExternals, visited, dead)
			go p.pool.Do(context, &pathBot{
				path:      pathURI.String(),
				index:     p.index,
				dead:      p.dead,
				wait:      p.wait,
				vl:        p.vl,
				visited:   p.visited,
				pool:      p.pool,
				externals: p.externals,
			})

		}
	}

}

//==============================================================================

// evaluatePath evalutes the giving URI path if valid and returns the status,
// a boolean indicating if its crawlable and a possible error if a failure
// occured.
func evaluatePath(path string) (status int, shouldCrawl bool, err error) {
	var res *http.Response

	res, err = httpClient.Head(path)
	if err != nil {

		// When an erro occurs, we get a nil response, so we have to print this out
		// and designated this as a failure and a dead link.
		fmt.Printf(`
URL: %s
Status: Failed to get HEAD for path
Error: %s

`, path, err.Error())

		status = http.StatusInternalServerError
		shouldCrawl = false
		return
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		status = res.StatusCode
		err = errors.New("Failed")
		shouldCrawl = false
		return
	}

	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		status = res.StatusCode
		err = nil
		shouldCrawl = false
		return
	}

	fmt.Printf(`
URL: %s
Status Code: %d

`, path, res.StatusCode)

	status = res.StatusCode
	err = nil
	shouldCrawl = true
	return
}

//==============================================================================

// farmLinks takes a given url and retrieves the needed links associated with
// that URL.
func farmLinks(url string, port chan string) error {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	// Collect all href links within the document. This way we can capture
	// external,internal and stylesheets within the page.
	hrefs := doc.Find("[href]")
	wg.Add(hrefs.Length())

	go hrefs.Each(func(index int, item *goquery.Selection) {
		defer wg.Done()
		href, ok := item.Attr("href")
		if !ok {
			return
		}

		if strings.Contains(href, "javascript:void(0)") {
			return
		}

		port <- href
	})

	// Collect all src links within the document. This way we can capture
	// documents and scripts within the page.
	srcs := doc.Find("[src]")
	wg.Add(srcs.Length())

	go srcs.Each(func(index int, item *goquery.Selection) {
		defer wg.Done()
		href, ok := item.Attr("href")
		if !ok {
			return
		}

		if strings.Contains(href, "javascript:void(0)") {
			return
		}

		port <- href
	})

	go func() {
		wg.Wait()
		close(port)
	}()

	return nil
}

//==============================================================================

// parsePath re-evaluates a giving path string using a root URL path, else
// returns an error if it fails or if the string is an invalid uri.
func parsePath(path string, index *url.URL) (*url.URL, error) {
	pathURI, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	if !pathURI.IsAbs() {
		pathURI = index.ResolveReference(pathURI)
	}

	return pathURI, nil
}
