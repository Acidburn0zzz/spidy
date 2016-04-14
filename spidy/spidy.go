package spidy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ardanlabs/kit/pool"
)

// Events defines an interface to allows the logging of events as they occur
// within the spidy API.
type Events interface {
	Event(context interface{}, event string, format string, data ...interface{})
	ErrorEvent(context interface{}, event string, err error, format string, data ...interface{})
}

//==============================================================================

// LinkReport defines a struct to entail failed links with their status and errors.
type LinkReport struct {
	Link   string
	Status int
	Error  error
}

// Run evaluates the given urlPath returning possible lists of deadlinks found
// within the page of the given link else returns a non-nil error if it failed.
func Run(context interface{}, urlPath string, all bool, workers int, events Events) ([]LinkReport, error) {
	events.Event(context, "Run", "Started : URL[%s] : Include Externals[%t] : Workers[%d]", urlPath, all, workers)

	path, err := url.Parse(urlPath)
	if err != nil {
		events.ErrorEvent(context, "Run", err, "Completed")
		return nil, err
	}

	// deadLinks provides a lists to contain all failed url links that are
	// considered deadlinks.
	var deadLinks []LinkReport

	dead := make(chan LinkReport)

	go collectFrom(path, all, workers, dead, events)

	for link := range dead {
		deadLinks = append(deadLinks, link)
	}

	events.Event(context, "Run", "Completed : Total Dead Links[%d]", len(deadLinks))
	return deadLinks, nil
}

//==============================================================================

// collectFrom uses a recursive function to map out the needed lists of links to.
// It returns a channel through which the acceptable links can be crawled from.
func collectFrom(path *url.URL, doExternals bool, maxWorkers int, dead chan LinkReport, events Events) {
	poolCfg := pool.Config{
		OptEvent:    pool.OptEvent{Event: events.Event},
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
		dead <- LinkReport{Link: path.String(), Status: status, Error: err}
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
	dead      chan LinkReport
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
			p.dead <- LinkReport{Link: p.path, Status: status, Error: err}
			return
		}

		if !crawleable {
			return
		}
	}

	links := make(chan string)

	if err := farmLinks(p.path, links); err != nil {
		fmt.Printf("Spidy Failed to Farm Links for Page[%s]: Error[%s]\n", p.path, err.Error())
		p.dead <- LinkReport{Link: p.path, Status: http.StatusInternalServerError, Error: err}
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

// httpClient to provide http requests abilities.
var httpClient = http.Client{
	Timeout: 30 * time.Second,
}

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

//==============================================================================
