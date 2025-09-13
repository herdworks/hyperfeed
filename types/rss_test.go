package hyperfeed

import (
	"os"
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

func TestRSSDetected(t *testing.T) {
	if !RSSDetected([]byte(sampleRSS)) {
		t.Fatalf("RSSDetect returned false")
	}
}

func TestRSSParse(t *testing.T) {
	rss, err := ParseRSS([]byte(sampleRSS))
	if err != nil {
		t.Fatalf("ParseRSS returned error: %v", err)
	}
	if rss.Channel.Title != "Example" {
		t.Fatalf("Channel title = %q; want %q", rss.Channel.Title, "Example")
	}
	if len(rss.Channel.Items) != 1 {
		t.Fatalf("Expected 1 item; got %d", len(rss.Channel.Items))
	}
	if rss.Channel.Items[0].Title != "First" {
		t.Fatalf("Item title = %q; want %q", rss.Channel.Items[0].Title, "First")
	}
}

func TestRSSParseTheTalkShow(t *testing.T) {
	body, err := os.ReadFile("testdata/2025-09-09/feeds/daringfireball.net.thetalkshow.rss.xml")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	rss, err := ParseRSS(body)
	if err != nil {
		t.Fatalf("ParseRSS returned error: %v", err)
	}
	if rss.Channel.Title == "" {
		t.Fatalf("Channel title is empty")
	}
	t.Logf("The Talk Show has %d items", len(rss.Channel.Items))
	if len(rss.Channel.Items) == 0 {
		t.Fatalf("Expected items; got 0")
	}
}
