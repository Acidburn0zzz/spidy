# Spidy
 Spidy provides a simple CLI tool for finding dead links on a target URI endpoint.
 It allows its users to check the status of all links only within or if desired
 in combination with the health of external links. It reports the status and state
 of the links and if any errors encountered, allowing the ability to discover
 dead or bad links.

## Install

  ```bash
  go get -u github.com/ardanlabs/spidy
  ```

## Usage
 Spidy provides two simple flags which provides allows setting the link to
 crawl and wether external links should be considered.

 ```bash

	// To crawl the giving url and no external links as well
	spidy -url http://golang.org
	spidy -url http://golang.org -hostOnly true

	// To crawl the giving url and external links as well
	spidy -url http://golang.org -hostOnly false


 ```
