package spidy_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/ardanlabs/kit/log"
	"github.com/ardanlabs/kit/tests"
	"github.com/ardanlabs/spidy/spidy"
)

//==============================================================================

var context = "testing"

func init() {
	log.Init(os.Stdout, func() int { return log.DEV }, log.Ldefault)
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

// var exts = regexp.MustCompile(".css|.png|.jpg|.js")

// TestSpidy tests the validity of the behaviour of the spidy crawler, using both
// positive and negative tests to validate the results.
func TestSpidy(t *testing.T) {
	t.Logf("Given the need to crawl pages with spidy")
	{

		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

			if req.URL.Path != "/" {
				if !validPaths[req.URL.Path] {
					res.WriteHeader(http.StatusNotFound)
					return
				}
			}

			ext := path.Ext(req.URL.Path)
			if ext != "" {
				res.Header().Set("Content-Type", "application/extension")
			}

			req.ParseForm()
			page := strings.TrimSpace(req.FormValue("page"))

			// fmt.Printf("Current Page: %s : %s\n", page, req.URL.Path)

			if page == "badlinks" {
				res.Write(ardanBadLink)
				return
			}

			if page == "badscripts" {
				// fmt.Println("Sending scripts: ", ardanBadScripts)
				res.Write(ardanBadScripts)
				return
			}

			if page == "badimages" {
				res.Write(ardanBadImages)
				return
			}

			res.WriteHeader(200)
			res.Write(ardan)

		}))

		defer server.Close()

		testValidPage(server.URL, t)
		testInvalidLinks(server.URL, t)
		testInvalidScripts(server.URL, t)
		testInvalidImages(server.URL, t)
	}
}

//==============================================================================

// testValidPage tests a positive path with our spidy crawler and assets all
// links within the page passes with no exceptions.
func testValidPage(url string, t *testing.T) {
	t.Logf("\tWhen retrieving a page with all valid links")
	{

		badlinks, err := spidy.Run(context, url, false, 30, -1, events)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, url, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, url)

		if len(badlinks) > 0 {
			t.Fatalf("\t%s\tShould have found no deadlinks in page[%s]: %+v", tests.Failed, url, badlinks)
		}
		t.Logf("\t%s\tShould have found no deadlinks in page[%s]", tests.Success, url)

	}
}

// testInvalidImages tests all images links within the giving pages and asserts
// we are able to catch all failing links.
func testInvalidImages(url string, t *testing.T) {
	t.Logf("\tWhen retrieving a valid page with invalid images")
	{

		url = fmt.Sprintf("%s?page=badimages", url)
		badlinks, err := spidy.Run(context, url, false, 30, -1, events)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, url, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, url)

		if len(badlinks) < 3 {
			t.Fatalf("\t%s\tShould have found 3 dead image links in page[%s]: %+v", tests.Failed, url, badlinks)
		}
		t.Logf("\t%s\tShould have found 3 dead image links in page[%s]", tests.Success, url)

	}
}

// testInvalidLinks tests all script/<link> links within the giving pages and asserts
// we are able to catch all failing links.
func testInvalidScripts(url string, t *testing.T) {
	t.Logf("\tWhen retrieving a valid page with invalid scripts")
	{
		url = fmt.Sprintf("%s?page=badscripts", url)
		badlinks, err := spidy.Run(context, url, false, 30, -1, events)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, url, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, url)

		if len(badlinks) < 2 {
			t.Fatalf("\t%s\tShould have found 2 dead script links in page[%s]: %+v", tests.Failed, url, badlinks)
		}
		t.Logf("\t%s\tShould have found 2 dead script links in page[%s]", tests.Success, url)
	}
}

// testInvalidLinks tests all href links within the giving pages and asserts
// we are able to catch all failing links.
func testInvalidLinks(url string, t *testing.T) {
	t.Logf("\tWhen retrieving a valid page with invalid hyperlinks")
	{
		url = fmt.Sprintf("%s?page=badlinks", url)
		badlinks, err := spidy.Run(context, url, false, 30, -1, events)
		if err != nil {
			t.Fatalf("\t%s\tShould have successfully retrieved page[%s]: %q", tests.Failed, url, err)
		}
		t.Logf("\t%s\tShould have successfully retrieved page[%s]", tests.Success, url)

		if len(badlinks) < 3 {
			t.Fatalf("\t%s\tShould have found 3 dead links in page[%s]: %+v", tests.Failed, url, badlinks)
		}
		t.Logf("\t%s\tShould have found 3 dead links in page[%s]", tests.Success, url)
	}
}

//==============================================================================
