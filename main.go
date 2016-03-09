package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackdanger/collectlinks"
)

// visited is a map for storing visited uri's to avoid visit loops.
var visited = make(map[string]bool)

// deadLinks provides a lists to contain all failed url links that are
// considered deadlinks.
var deadLinks []string

// httpClient to provide http requests abilities.
var httpClient http.Client

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

	queue := make(chan string)

	if *hostOnly {
		fmt.Printf("Crawling URL[%s]\n", *link)
	} else {
		fmt.Printf("Crawling All Links in URL[%s]\n", *link)
	}

	// Go routine puts url flag into channel for queuing.
	go func() {
		queue <- *link
	}()

	hostURL, err := url.Parse(*link)
	if err != nil {
		return
	}

	for {
		select {
		case uri, ok := <-queue:
			if !ok {
				return
			}

			queueLinks(hostURL.Host, uri, queue, *hostOnly)

		case <-time.After(1 * time.Minute):
			fmt.Println("Ending crawler. Goodbye!")
			fmt.Println("--------------------DEAD LINKS------------------------------")

			for _, f := range deadLinks {
				fmt.Println(f)
			}

			fmt.Println("------------------------------------------------------------")

			return
		}
	}

}

// queueLinks is used for making http calls to the queued links in queue channel

func queueLinks(host, uri string, queue chan string, boolflag bool) {
	if visited[uri] {
		return
	}

	crawledURL, _ := url.Parse(uri)

	if boolflag {
		if !strings.Contains(crawledURL.Host, host) {
			return
		}
	}

	fmt.Printf("Fetching: %s\n", uri)

	// Store and tag URI as visited.
	visited[uri] = true

	resp, err := httpClient.Get(uri)
	if err != nil {
		fmt.Printf(`Host: %s
URI: %s
Error: %s

`, host, uri, err.Error())

		deadLinks = append(deadLinks, uri)
		return
	}

	var failedCodes = []int{
		http.StatusUnauthorized,
		http.StatusBadGateway,
		http.StatusBadRequest,
		http.StatusUnavailableForLegalReasons,
		http.StatusUnauthorized,
		http.StatusServiceUnavailable,
		http.StatusNotFound,
		http.StatusForbidden,
		http.StatusMovedPermanently,
	}

	for _, failed := range failedCodes {
		if failed != resp.StatusCode {
			continue
		}

		fmt.Printf(`Host: %s
URI: %s
Error: %s

`, host, uri, fmt.Sprintf("Response Status Code[%d]", resp.StatusCode))

		deadLinks = append(deadLinks, uri)
		return
	}

	fmt.Printf("Status: URL[%s] OK!\n\n", uri)

	defer resp.Body.Close()

	// Collectlinks package helps in parsing a webpage & returning found
	// hyperlink href.
	links := collectlinks.All(resp.Body)

	for _, link := range links {

		absolute, err := fixURL(link, uri)
		if err != nil {
			continue
		}

		// Don't queue a uri twice.
		go func() {
			queue <- absolute
		}()
	}
}

// fix url combines the pathname with a base url,returns error if the failed.
func fixURL(href, base string) (string, error) {
	uri, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	uri = baseURL.ResolveReference(uri)

	return uri.String(), err
}
