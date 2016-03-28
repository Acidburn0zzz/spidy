package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// httpClient to provide http requests abilities.
var httpClient = http.Client{
	Timeout: 30 * time.Second,
}

type linkReport struct {
	Link   string
	Status int
}

func main() {
	flag.Usage = func() {
		fmt.Println(`
Spidy - A simple deadlink finder.

Flags:

 -url "URL to crawl for dead links"
 -hostOnly "A boolean flag which allows setting whether external links should be considered"

Usage:

	// To crawl the giving url and no external links as well
	spidy -url http://golang.org
	spidy -url http://golang.org -hostOnly true

	// To crawl the giving url and external links as well
	spidy -url http://golang.org -hostOnly false

`)
	}

	var link = flag.String("url", "", "Url to crawl for dead links")
	var hostOnly = flag.Bool("hostOnly", true, "flag to crawl none base host. Defaults to true")

	flag.Parse()

	if *hostOnly {
		fmt.Printf("Crawling URL[%s]\n", *link)
	} else {
		fmt.Printf("Crawling All Links in URL[%s]\n", *link)
	}

	path, err := url.Parse(*link)
	if err != nil {
		fmt.Printf("Error: Unparsable URI: %s\n", *link)
		os.Exit(-1)
		return
	}

	// deadLinks provides a lists to contain all failed url links that are
	// considered deadlinks.
	var deadLinks []*linkReport

	dead := collectFrom(path, !*hostOnly)

	for {
		select {
		case link, ok := <-dead:
			if !ok {
				fmt.Println("--------------------DEAD LINKS------------------------------")

				if len(deadLinks) > 0 {
					for _, f := range deadLinks {
						fmt.Printf(`
			URL: %s
			Status Code: %d

			`, f.Link, f.Status)
					}
				} else {
					fmt.Println("No Dead Links Found! Woot!")
				}

				fmt.Println("------------------------------------------------------------")

				os.Exit(-1)
				return
			}

			deadLinks = append(deadLinks, link)
		}
	}

}

// farmLinks takes a given url and retrieves the needed links associated with
// that URL.
func farmLinks(url string) ([]string, error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return nil, err
	}

	var links []string

	// Collect all href links within the document. This way we can capture
	// external,internal and stylesheets within the page.
	doc.Find("[href]").Each(func(index int, item *goquery.Selection) {
		href, ok := item.Attr("href")
		if !ok {
			return
		}
		links = append(links, href)
	})

	// Collect all src links within the document. This way we can capture
	// documents and scripts within the page.
	doc.Find("[src]").Each(func(index int, item *goquery.Selection) {
		href, ok := item.Attr("href")
		if !ok {
			return
		}
		links = append(links, href)
	})

	return links, nil
}

// collectFrom uses a recursive function to map out the needed lists of links to.
// It returns a channel through which the acceptable links can be crawled from.
func collectFrom(path *url.URL, doExternals bool) <-chan *linkReport {
	dead := make(chan *linkReport)

	// visited is a map for storing visited uri's to avoid visit loops.
	visited := make(map[string]bool)

	go func() {
		defer close(dead)
		collect(path.String(), path, doExternals, visited, dead)
	}()

	return dead
}

func collect(path string, host *url.URL, doExternals bool, visited map[string]bool, dead chan *linkReport) {

	// Add to our visited lists.
	visited[path] = true

	res, err := httpClient.Head(path)
	if err != nil {
		// TODO: Won't res be nil when an error is received here, need to confirm.
		dead <- &linkReport{Link: path, Status: res.StatusCode}
		return
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		dead <- &linkReport{Link: path, Status: res.StatusCode}
		return
	}

	links, err := farmLinks(path)
	if err != nil {
		fmt.Printf("Spidy Failed to Farm Links for Page[%s]: Error[%s]\n", path, err.Error())
		return
	}

	fmt.Printf(`
URL: %s
Status Code: %d

`, path, res.StatusCode)

	for _, link := range links {
		if visited[link] {
			continue
		}

		// If its a hash based path, then just add it as seen and skip.
		if strings.HasPrefix(link, "#") {
			visited[link] = true
			continue
		}

		pathURI, err := url.Parse(link)
		if err != nil {
			continue
		}

		// If we are running into the same root link, skip it.
		if strings.TrimSpace(link) == "/" {
			continue
		}

		if !pathURI.IsAbs() {
			pathURI = host.ResolveReference(pathURI)
		}

		// If we are are not allowed external links, then check and if not
		// within host then skip but add it to scene list, we dont, want
		// to go through the same link twice.
		if !doExternals && !strings.Contains(pathURI.Host, host.Host) {
			visited[link] = true
			continue
		}

		// Although we have fixed the path, we still need to add it down into the list
		// of visited.
		visited[link] = true

		collect(pathURI.String(), host, doExternals, visited, dead)
	}

}
