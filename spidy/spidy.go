package spidy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/html"

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

// Config defines the configuration through which our crawler defines its running
// parameters.
type Config struct {
	Client  *http.Client
	URL     string
	All     bool
	Workers int
	Depth   int
	Events  Events
}

// Run evaluates the given urlPath returning possible lists of deadlinks found
// within the page of the given link else returns a non-nil error if it failed.
func Run(context interface{}, c *Config) ([]LinkReport, error) {
	c.Events.Event(context, "Run", "Started : URL[%s] : Include Externals[%t] : Workers[%d] : HTTPTimeout[%s]", c.URL, c.All, c.Workers, c.Client.Timeout)

	path, err := url.Parse(c.URL)
	if err != nil {
		c.Events.ErrorEvent(context, "Run", err, "Completed")
		return nil, err
	}

	// deadLinks provides a lists to contain all failed url links that are
	// considered deadlinks.
	var deadLinks []LinkReport

	dead := make(chan LinkReport)

	go collectFrom(c, path, dead)

	for link := range dead {
		deadLinks = append(deadLinks, link)
	}

	c.Events.Event(context, "Run", "Completed : Total Dead Links[%d]", len(deadLinks))
	return deadLinks, nil
}

//==============================================================================

var depths int64

// collectFrom uses a recursive function to map out the needed lists of links to.
// It returns a channel through which the acceptable links can be crawled from.
func collectFrom(c *Config, path *url.URL, dead chan LinkReport) {
	poolCfg := pool.Config{
		OptEvent:    pool.OptEvent{Event: c.Events.Event},
		MinRoutines: func() int { return 10 },
		MaxRoutines: func() int { return c.Workers },
	}

	defer close(dead)

	// create a new worker pool.
	pl, err := pool.New("spidy", "collectFrom", poolCfg)
	if err != nil {
		fmt.Printf("Spidy failed to create work pool: %s\n", err.Error())
		return
	}

	defer pl.Shutdown("spidy")

	// Evalue the giving path and check if its a crawlable endpoint and
	// if the status meets our criteria.
	status, crawleable, err := evaluatePath(path.String(), c)
	// fmt.Println("First::Evaluate: ", path, " Status: ", status)
	if err != nil {
		dead <- LinkReport{Link: path.String(), Status: status, Error: err}
		return
	}

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

	pl.Do("collectFrom", &pathBot{
		config:    c,
		path:      path.String(),
		index:     path,
		dead:      dead,
		wait:      &wait,
		vl:        &vl,
		visited:   visited,
		pool:      pl,
		externals: c.All,
		skipCheck: true,
		maxdepths: c.Depth,
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
	config    *Config
	wait      *sync.WaitGroup
	vl        *sync.RWMutex
	visited   map[string]bool
	pool      *pool.Pool
	index     *url.URL
	skipCheck bool
	externals bool
	maxdepths int
	cd        int
}

// Work performs the necessary tasks of validating a link and rescheduling
// checks for sublinks.
func (p *pathBot) Work(context interface{}, id int) {
	defer p.wait.Done()

	p.vl.Lock()
	p.visited[p.path] = true
	p.vl.Unlock()

	if !p.skipCheck {
		status, crawleable, err := evaluatePath(p.path, p.config)
		// fmt.Println("Evaluate: ", p.path, " Status: ", status)
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
		// fmt.Printf("Spidy Failed to Farm Links for Page[%s]: Error[%s]\n", p.path, err.Error())
		p.dead <- LinkReport{Link: p.path, Status: http.StatusInternalServerError, Error: err}
		return
	}

	for {
		select {
		case link, ok := <-links:
			if !ok {
				atomic.AddInt64(&depths, 1)
				return
			}

			if p.maxdepths > 0 && int(atomic.LoadInt64(&depths)) > p.maxdepths {
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

			// fmt.Println("Collect: ", pathURI.String())

			p.pool.Do(context, &pathBot{
				path:      pathURI.String(),
				config:    p.config,
				index:     p.index,
				dead:      p.dead,
				wait:      p.wait,
				vl:        p.vl,
				visited:   p.visited,
				pool:      p.pool,
				externals: p.externals,
				maxdepths: p.maxdepths,
			})
		}
	}

}

//==============================================================================

// evaluatePath evalutes the giving URI path if valid and returns the status,
// a boolean indicating if its crawlable and a possible error if a failure
// occured.
func evaluatePath(path string, c *Config) (status int, shouldCrawl bool, err error) {
	var res *http.Response

	res, err = c.Client.Head(path)
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

	status = res.StatusCode
	shouldCrawl = false

	if res.StatusCode < 200 || res.StatusCode > 299 {
		err = errors.New("Link Failed")
		return
	}

	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		status = res.StatusCode
		return
	}

	fmt.Printf(`
URL: %s
Status Code: %d

`, path, res.StatusCode)

	shouldCrawl = true
	return
}

//==============================================================================

// getAttr returns the giving attribute for a specific name type if found.
func getAttr(attrs []html.Attribute, key string) (attr html.Attribute, found bool) {
	for _, attr = range attrs {
		if attr.Key == key {
			found = true
			return
		}
	}
	return
}

// farmLinks takes a given url and retrieves the needed links associated with
// that URL.
func farmLinks(url string, port chan string) error {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return err
	}

	// Collect all href links within the document. This way we can capture
	// external,internal and stylesheets within the page.
	hrefs := doc.Find("[href]")
	srcs := doc.Find("[src]")

	hrefLen := hrefs.Length()
	srcLen := srcs.Length()

	total := hrefLen

	if total < srcLen {
		total = srcLen
	}

	go func() {
		defer close(port)

		for i := 0; i < total; i++ {
			if i < hrefLen {
				item, ok := getAttr(hrefs.Get(i).Attr, "href")
				if !ok {
					continue
				}

				if strings.Contains(item.Val, "javascript:void(0)") {
					continue
				}

				port <- item.Val
			}

			if i < srcLen {
				item, ok := getAttr(srcs.Get(i).Attr, "src")
				if !ok {
					continue
				}

				if strings.Contains(item.Val, "javascript:void(0)") {
					continue
				}

				port <- item.Val
			}
		}

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
