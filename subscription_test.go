package hyperfeed

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestSubscriptionListParse(t *testing.T) {
	var subscriptionList SubscriptionList
	var data []byte
	var err error
	const path = "testdata/2025-09-09/nnw_default_subscriptions.opml"
	if data, err = os.ReadFile(path); err != nil {
		t.Fatalf("Failed to read subscription list: %v", err)
	}
	if err = subscriptionList.Parse(data); err != nil {
		t.Fatalf("Failed to parse subscription list: %v", err)
	}
	if len(subscriptionList.OPML.Body.Outlines) != 14 {
		t.Fatalf("Expected 14 outlines; got %d", len(subscriptionList.OPML.Body.Outlines))
	}
}

// TestSubscriptionListInitFeeds verifies the subscription-list pattern rules
// (as distinct from the OPML spec): outlines become Feeds, and non-web
// (e.g. file://) feed URLs are skipped so OPML import can never introduce a
// local-file feed.
func TestSubscriptionListInitFeeds(t *testing.T) {
	const opmlDoc = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
	<head><title>Mixed</title></head>
	<body>
		<outline text="Local" type="rss" xmlUrl="file:///etc/passwd" htmlUrl="https://example.com/"/>
		<outline text="Remote" type="rss" xmlUrl="https://example.com/feed.xml" htmlUrl="https://example.com/"/>
	</body>
</opml>`
	var subscriptionList SubscriptionList
	if err := subscriptionList.Parse([]byte(opmlDoc)); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	feeds, err := subscriptionList.InitFeeds(io.Discard)
	if err != nil {
		t.Fatalf("InitFeeds() error = %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("InitFeeds() feeds = %d; want 1 (file:// outline must be skipped)", len(feeds))
	}
	if got := feeds[0].URL.URL().Scheme; got != "https" {
		t.Errorf("InitFeeds() surviving feed scheme = %q; want https", got)
	}
	if got := feeds[0].Title; got != "Remote" {
		t.Errorf("InitFeeds() surviving feed title = %q; want %q", got, "Remote")
	}
}

func TestSubscriptionListInitFeedsNestedOutlines(t *testing.T) {
	const opmlDoc = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
	<head><title>Blogroll</title></head>
	<body>
		<outline text="Blogroll" title="Blogroll">
			<outline text="First" type="rss" xmlUrl="https://example.com/first.xml" htmlUrl="https://example.com/"/>
			<outline text="Nested Local" type="rss" xmlUrl="file:///etc/passwd"/>
			<outline text="Folder">
				<outline text="Second" type="rss" xmlUrl="https://example.org/second.xml" htmlUrl="https://example.org/"/>
			</outline>
		</outline>
	</body>
</opml>`
	var subscriptionList SubscriptionList
	if err := subscriptionList.Parse([]byte(opmlDoc)); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	feeds, err := subscriptionList.InitFeeds(io.Discard)
	if err != nil {
		t.Fatalf("InitFeeds() error = %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("InitFeeds() feeds = %d; want 2", len(feeds))
	}
	if got := feeds[0].Title; got != "First" {
		t.Errorf("feeds[0].Title = %q; want %q", got, "First")
	}
	if got := feeds[0].URL.String(); got != "https://example.com/first.xml" {
		t.Errorf("feeds[0].URL = %q; want %q", got, "https://example.com/first.xml")
	}
	if got := feeds[1].Title; got != "Second" {
		t.Errorf("feeds[1].Title = %q; want %q", got, "Second")
	}
	if got := feeds[1].URL.String(); got != "https://example.org/second.xml" {
		t.Errorf("feeds[1].URL = %q; want %q", got, "https://example.org/second.xml")
	}
}

func TestSubscriptionListInitFeedsTrimsOutlineURLs(t *testing.T) {
	const opmlDoc = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
	<head><title>Whitespace</title></head>
	<body>
		<outline text="SCOTUSblog" type="rss" xmlUrl=" https://scotusblog.com/feed" htmlUrl=" https://www.scotusblog.com/"/>
	</body>
</opml>`
	var subscriptionList SubscriptionList
	if err := subscriptionList.Parse([]byte(opmlDoc)); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	feeds, err := subscriptionList.InitFeeds(io.Discard)
	if err != nil {
		t.Fatalf("InitFeeds() error = %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("InitFeeds() feeds = %d; want 1", len(feeds))
	}
	if got := feeds[0].URL.String(); got != "https://scotusblog.com/feed" {
		t.Errorf("feed url = %q; want %q", got, "https://scotusblog.com/feed")
	}
	if got := feeds[0].HTMLURL.String(); got != "https://www.scotusblog.com/" {
		t.Errorf("feed htmlUrl = %q; want %q", got, "https://www.scotusblog.com/")
	}
}

// TestSubscriptionListMarshalOPML verifies the export side of the pattern:
// Feeds render as subscription outlines, and the output round-trips through
// Parse.
func TestSubscriptionListMarshalOPML(t *testing.T) {
	const opmlDoc = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
	<head><title>Round Trip</title></head>
	<body>
		<outline text="Remote" type="rss" xmlUrl="https://user:secret@example.com/feed.xml" htmlUrl="https://example.com/"/>
	</body>
</opml>`
	var subscriptionList SubscriptionList
	if err := subscriptionList.Parse([]byte(opmlDoc)); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	feeds, err := subscriptionList.InitFeeds(io.Discard)
	if err != nil {
		t.Fatalf("InitFeeds() error = %v", err)
	}
	subscriptionList.Feeds = feeds

	out, err := subscriptionList.MarshalOPML(false)
	if err != nil {
		t.Fatalf("MarshalOPML() error = %v", err)
	}
	var roundTrip SubscriptionList
	if err := roundTrip.Parse(out); err != nil {
		t.Fatalf("Parse(MarshalOPML()) error = %v", err)
	}
	if roundTrip.Title != "Round Trip" {
		t.Errorf("round-trip title = %q; want %q", roundTrip.Title, "Round Trip")
	}
	if got := len(roundTrip.OPML.Body.Outlines); got != 1 {
		t.Fatalf("round-trip outlines = %d; want 1", got)
	}
	if got := roundTrip.OPML.Body.Outlines[0].XMLURL; !strings.Contains(got, "secret") {
		t.Errorf("MarshalOPML(redacted=false) xmlUrl = %q; want credentials preserved", got)
	}

	redacted, err := subscriptionList.MarshalOPML(true)
	if err != nil {
		t.Fatalf("MarshalOPML(redacted) error = %v", err)
	}
	if strings.Contains(string(redacted), "secret") {
		t.Errorf("MarshalOPML(redacted=true) output still contains credentials")
	}
}
