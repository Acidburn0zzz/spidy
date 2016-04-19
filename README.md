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

- Environment Variables
  Spidy allows delpoyment of its binary using environment variables
  which sets the target and flags for which the tool will crawl.

  - SPIDY_TARGET_URL
     This sets the target URL which the client target for crawling

  - SPIDY_EXTERNAL_LINKS
     This sets the external flags which ensures allows crawling
     hosts from that of the target

  - SPIDY_MAX_WORKERS
     This sets the maximum workers to use for its operation.

 ```bash
  > export SPIDY_MAX_WORKERS=300
  > export SPIDY_TARGET_URL="https://ardanlabs.com"
  > export SPIDY_EXTERNAL_LINKS=false

	> spidy


 ```

- CLI
 Spidy provides two simple flags which sets the target URL to
 crawl and flips the flat to allow external links from the target host.

 ```bash

	// To crawl the giving url and no external links as well, using 120 workers
	spidy -url http://golang.org -w 120

	// To crawl the giving url and external links as well
	spidy -url http://golang.org -externals true


 ```
