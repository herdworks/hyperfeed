package hyperfeed

import (
	"net/url"
	"os"
	"testing"
)

func TestDetectKind(t *testing.T) {
	var sampleDataFileURL *url.URL
	var subscriptionList SubscriptionList
	var err error
	sampleDataFileURL = &url.URL{ Path: "testdata/2025-09-09/nnw_default_subscriptions.opml" }
	subscriptionList = SubscriptionList{URI: sampleDataFileURL}
	if err = subscriptionList.Load(); err != nil {
		t.Fatalf("Failed to load subscription list: %v", err)
	}
	if len(subscriptionList.OPML.Body.Outlines) != 14 {
		t.Fatalf("Expected 14 outlines; got %d", len(subscriptionList.OPML.Body.Outlines))
	}

	// list files in testdata/2025-09-09/feeds/ and try to parse each one
	testdatadir := "testdata/2025-09-09/feeds/"
	files, err := os.ReadDir(testdatadir)
	if err != nil {
		t.Fatalf("Failed to read test data directory: %v", err)
	}
	for _, f := range files {
		if f.IsDir() { continue }
		feedBytes, err := os.ReadFile(testdatadir + f.Name())
		if err != nil {
			t.Fatalf("Failed to read feed file %q: %v", f.Name(), err)
		}
		var feed Feed
		feedKind := feed.DetectKind(feedBytes)
		if feedKind == "" {
			t.Logf("Could not detect kind for feed file %q", f.Name())
			continue
		}
		if err = feed.Parse(feedKind, feedBytes); err != nil {
			t.Logf("Parse error for feed file %q: %v", f.Name(), err)
			continue
		}
		if feed.RawFeed == nil {
			t.Logf("No RawFeed for %q feed file %q", feedKind, f.Name())
		}
		t.Logf("Kind %q for feed file %q (%d items)", feedKind, f.Name(), feed.RawFeed.ItemCount())
	}
}
