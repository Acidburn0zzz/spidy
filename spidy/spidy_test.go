package spidy_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ardanlabs/kit/log"
	"github.com/ardanlabs/kit/tests"
	"github.com/ardanlabs/spidy/spidy"
)

//==============================================================================

var context = "testing"

func init() {
	tests.Init("")
}

//==============================================================================

var events Events

// Events defines a event logger.
type Events struct{}

// ErrorEvent logs error message for events.
func (Events) ErrorEvent(context interface{}, event string, err error, format string, data ...interface{}) {
	log.Error(context, event, err, format, data...)
}

// Event logs standard message for events.
func (Events) Event(context interface{}, event string, format string, data ...interface{}) {
	log.Dev(context, event, format, data...)
}

//==============================================================================

// TestSpidy tests the validity of the behaviour of the spidy crawler, using both
// positive and negative tests to validate the results.
func TestSpidy(t *testing.T) {
	tests.ResetLog()
	defer tests.DisplayLog()

	t.Logf("Given the need to crawl pages with spidy")
	{

		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if req.URL.Path != "/" {
				if !validPaths[req.URL.Path] {
					res.WriteHeader(http.StatusNotFound)
					return
				}
			}

			req.ParseForm()
			page := strings.TrimSpace(req.FormValue("page"))

			switch page {
			case "badscripts":
				fmt.Println("Sending scripts: ", req.URL)
				res.Write(ardanBadScripts)
				return
			case "badlinks":
				res.Write(ardanBadLink)
				return
			case "badimages":
				res.Write(ardanBadImages)
				return
			}

			res.WriteHeader(200)
			res.Write(ardan)

		}))

		defer server.Close()

		conf := spidy.Config{
			Client:  &http.Client{Timeout: time.Duration(30000) * time.Millisecond},
			URL:     server.URL,
			All:     false,
			Workers: 30,
			Depth:   -1,
			Events:  events,
		}

		testValidPage(conf, t)
		testInvalidScripts(conf, t)
		testInvalidLinks(conf, t)
		testInvalidImages(conf, t)
	}
}

//==============================================================================

// testValidPage tests a positive path with our spidy crawler and assets all
// links within the page passes with no exceptions.
func testValidPage(c spidy.Config, t *testing.T) {
	t.Logf("\tWhen retrieving a page with all valid links")
	{

		badlinks, err := spidy.Run(context, &c)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, c.URL, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, c.URL)

		if len(badlinks) > 0 {
			t.Fatalf("\t%s\tShould have found no deadlinks in page[%s]: %+v", tests.Failed, c.URL, badlinks)
		}
		t.Logf("\t%s\tShould have found no deadlinks in page[%s]", tests.Success, c.URL)

	}
}

// testInvalidImages tests all images links within the giving pages and asserts
// we are able to catch all failing links.
func testInvalidImages(c spidy.Config, t *testing.T) {
	t.Logf("\tWhen retrieving a valid page with invalid images")
	{

		c.URL = fmt.Sprintf("%s?page=badimages", c.URL)
		badlinks, err := spidy.Run(context, &c)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, c.URL, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, c.URL)

		if len(badlinks) < 3 {
			t.Fatalf("\t%s\tShould have found 3 dead image links in page[%s]: %+v", tests.Failed, c.URL, badlinks)
		}
		t.Logf("\t%s\tShould have found 3 dead image links in page[%s]", tests.Success, c.URL)

	}
}

// testInvalidLinks tests all script/<link> links within the giving pages and asserts
// we are able to catch all failing links.
func testInvalidScripts(c spidy.Config, t *testing.T) {
	t.Logf("\tWhen retrieving a valid page with invalid scripts")
	{
		c.URL = fmt.Sprintf("%s?page=badscripts", c.URL)
		badlinks, err := spidy.Run(context, &c)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, c.URL, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, c.URL)

		if len(badlinks) < 2 {
			t.Fatalf("\t%s\tShould have found 2 dead script links in page[%s]: %+v", tests.Failed, c.URL, badlinks)
		}
		t.Logf("\t%s\tShould have found 2 dead script links in page[%s]", tests.Success, c.URL)
	}
}

// testInvalidLinks tests all href links within the giving pages and asserts
// we are able to catch all failing links.
func testInvalidLinks(c spidy.Config, t *testing.T) {
	t.Logf("\tWhen retrieving a valid page with invalid hyperlinks")
	{
		c.URL = fmt.Sprintf("%s?page=badlinks", c.URL)
		badlinks, err := spidy.Run(context, &c)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, c.URL, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, c.URL)

		if len(badlinks) < 3 {
			t.Fatalf("\t%s\tShould have found 3 dead links in page[%s]: %+v", tests.Failed, c.URL, badlinks)
		}
		t.Logf("\t%s\tShould have found 3 dead links in page[%s]", tests.Success, c.URL)
	}
}

//==============================================================================
