package hyperfeed

import (
	"net/url"
	"os"
	"testing"

	"github.com/herdworks/hyperfeed-go/internal/testkit"
	"github.com/herdworks/hyperfeed-go/transporter"
)

func TestDetectKind(t *testing.T) {
	var subscriptionList SubscriptionList
	var sampleDataFile *os.File
	var sampleDataFileURL *url.URL
	var err error
	if sampleDataFile, err = os.CreateTemp("/tmp/", "default.opml"); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(sampleDataFile.Name())
	if _, err := sampleDataFile.Write([]byte(testkit.NNWDefaultOPML)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if sampleDataFileURL, err = url.Parse(sampleDataFile.Name()); err != nil {
		t.Fatalf("Failed to parse sampleDataFile URL: %v", err)
	}
	subscriptionList = SubscriptionList{URI: sampleDataFileURL}
	if err = subscriptionList.Load(); err != nil {
		t.Fatalf("Failed to load subscription list: %v", err)
	}
	if len(subscriptionList.OPML.Body.Outlines) != 14 {
		t.Fatalf("Expected 14 outlines; got %d", len(subscriptionList.OPML.Body.Outlines))
	}

	transporter := transporter.NewTransporter()
	for _, outline := range subscriptionList.OPML.Body.Outlines {
		xmlurl, err := url.Parse(outline.XMLURL)
		if err != nil {
			t.Fatalf("Failed to parse outline XMLURL %q: %v", outline.XMLURL, err)
		}
		feed := Feed{Title: outline.Title, URL: xmlurl}
		feedBytes, err := transporter.Get(feed.URL.String())
		if err != nil {
			t.Logf("Fetch error for %q: %v", feed.URL.String(), err)
			continue
		}
		feedKind := feed.DetectKind(feedBytes)
		t.Logf("Kind %q for %q", feedKind, feed.URL.String())
	}
}
