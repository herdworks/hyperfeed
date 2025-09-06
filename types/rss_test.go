package hyperfeed

import (
	"net/url"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Example</title>
<link>http://example.com/</link>
<description>Sample</description>
<item>
<title>First</title>
<link>http://example.com/1</link>
<guid>1</guid>
<pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
</item>
</channel>
</rss>`

func TestRSSDetect(t *testing.T) {
	if !RSSDetect([]byte(sampleRSS)) {
		t.Fatalf("RSSDetect returned false")
	}
}

func TestRSSParse(t *testing.T) {
	base, _ := url.Parse("http://example.com/feed")
	res, err := RSSParse([]byte(sampleRSS), RSSOptions{BaseURL: base})
	if err != nil {
		t.Fatalf("RSSParse returned error: %v", err)
	}
	if res.Feed.Title != "Example" {
		t.Fatalf("Feed title = %q; want %q", res.Feed.Title, "Example")
	}
	if len(res.Items) != 1 {
		t.Fatalf("Expected 1 item; got %d", len(res.Items))
	}
	if res.Items[0].Title != "First" {
		t.Fatalf("Item title = %q; want %q", res.Items[0].Title, "First")
	}
}
