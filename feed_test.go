package hyperfeed

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed/atom"
	"github.com/herdworks/hyperfeed/href"
	"github.com/herdworks/hyperfeed/jsonfeed"
	"github.com/herdworks/hyperfeed/rss"
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

func TestFeedPublishingFieldsJSON(t *testing.T) {
	var feed Feed
	if err := json.Unmarshal([]byte(`{"url":"https://example.com/feed.xml"}`), &feed); err != nil {
		t.Fatal(err)
	}
	if !feed.ModifiedAt.IsZero() || feed.Bylines != nil || feed.Links != nil || feed.CoverURL != nil || feed.Categories != nil || feed.Medium != "" {
		t.Fatal("legacy JSON must leave feed publishing fields unspecified")
	}
	assertJSONOmitted(t, feed, "modified_at", "bylines", "links", "cover_url", "categories", "medium")

	zero := int64(0)
	feed.ModifiedAt = time.Date(2026, 9, 22, 12, 30, 0, 0, time.UTC)
	feed.Bylines = []Byline{{Name: "Editor", Role: "editor", Order: 7}}
	feed.Links = []*Link{{Rel: "self", Mimetype: "application/atom+xml", HrefLang: "en", Length: &zero, Origin: "metadata"}}
	feed.Categories = []Category{NewCategory("Open Source")}
	feed.Medium = "podcast"
	coverURL, err := href.Parse("https://example.com/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	feed.CoverURL = coverURL
	data, err := json.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Feed
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(feed, decoded) {
		t.Fatalf("round trip changed supplied values:\n%s\n%#v", data, decoded)
	}
}

func TestFeedPublishingFieldsDoNotChangeIdentityOrDatabaseValues(t *testing.T) {
	u, err := href.Parse("https://example.com/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	feed := Feed{URL: u}
	feedID, feedHash, feedValues := feed.Identifier(), feed.Hash(), feed.DatabaseInsertValues()
	feed.Bylines = []Byline{{Name: "Editor", Role: "editor", Order: 1}}
	feed.Links = []*Link{{Href: u, Rel: "self", Mimetype: "application/atom+xml"}}
	feed.ModifiedAt = time.Date(2026, 9, 22, 12, 30, 0, 0, time.UTC)
	feed.Categories = []Category{NewCategory("Open Source")}
	feed.Medium = "podcast"
	feed.CoverURL, err = href.Parse("https://example.com/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	if feed.Identifier() != feedID || feed.Hash() != feedHash || !reflect.DeepEqual(feedValues, feed.DatabaseInsertValues()) {
		t.Fatal("publishing metadata changed feed identity or persistence")
	}
}

func assertJSONOmitted(t *testing.T, value any, keys ...string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if _, exists := object[key]; exists {
			t.Errorf("unspecified %s present in %s", key, data)
		}
	}
}
