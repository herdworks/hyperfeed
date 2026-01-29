package hyperfeed

import (
	"os"
	"testing"

	"github.com/herdworks/hyperfeed-go/types/atom"
	href "github.com/herdworks/hyperfeed-go/types/href"
	"github.com/herdworks/hyperfeed-go/types/jsonfeed"
	"github.com/herdworks/hyperfeed-go/types/rss"
)

func TestDetectKind(t *testing.T) {
	var testdatadir string = "testdata/2025-09-09/feeds/" // list files in testdata/2025-09-09/feeds/ and try to parse each one
	var files []os.DirEntry
	var err error
	if files, err = os.ReadDir(testdatadir); err != nil {
		t.Fatalf("Failed to read test data directory: %v", err)
	}
	for _, f := range files {
		var feed *Feed = &Feed{}
		var feedKind string
		if f.IsDir() {
			continue
		}
		if feed.Bytes, err = os.ReadFile(testdatadir + f.Name()); err != nil {
			t.Fatalf("Failed to read feed file %q: %v", f.Name(), err)
		}
		switch {
		case rss.Detected(feed.Bytes):
			feedKind = "rss"
		case atom.Detected(feed.Bytes):
			feedKind = "atom"
		case jsonfeed.Detected(feed.Bytes):
			feedKind = "jsonfeed"
		default:
			t.Logf("Could not detect kind for feed file %q", f.Name())
		}
		t.Logf("Detected feed kind %q for feed file %q", feedKind, f.Name())
	}
}

func TestWebsiteURLAndIdentifier(t *testing.T) {
	feed := &Feed{
		HTMLURL: &href.Href{
			Scheme:   "https",
			Host:     "example.com",
			Path:     "/posts/hello",
			RawQuery: "utm_source=newsletter&utm_medium=email&keep=this",
			Fragment: "section",
		},
	}

	website := feed.WebsiteURL()
	if website == nil {
		t.Fatal("expected WebsiteURL to be non-nil")
	}
	if website.Path != "/" {
		t.Fatalf("expected website path '/', got %q", website.Path)
	}
	if website.RawQuery != "" {
		t.Fatalf("expected website query to be empty, got %q", website.RawQuery)
	}
	if website.Fragment != "" {
		t.Fatalf("expected website fragment to be empty, got %q", website.Fragment)
	}

	feedRoot := &Feed{
		HTMLURL: &href.Href{
			Scheme: "https",
			Host:   "example.com",
			Path:   "/",
		},
	}
	if feed.WebsiteIdentifier() != feedRoot.WebsiteIdentifier() {
		t.Fatalf("expected WebsiteIdentifier to match for root and path URLs")
	}

	feedEmpty := &Feed{}
	if feedEmpty.WebsiteURL() != nil {
		t.Fatal("expected WebsiteURL to be nil for missing HTMLURL")
	}
	if feedEmpty.WebsiteIdentifier() == "" {
		t.Fatalf("expected WebsiteIdentifier to return unknown identifier for missing HTMLURL")
	}
}
