package hyperfeed

import (
	"testing"

	"github.com/herdworks/hyperfeed-go/transporter"
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
	transporter := transporter.NewTransporter()
	t.Logf("Fetching  https://daringfireball.net/thetalkshow/rss")
	body, err := transporter.Get("https://daringfireball.net/thetalkshow/rss")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	t.Logf("200 OK    https://daringfireball.net/thetalkshow/rss")
	rss, err := ParseRSS(body)
	if err != nil {
		t.Fatalf("ParseRSS returned error: %v", err)
	}
	if rss.Channel.Title != "The Talk Show With John Gruber" {
		t.Fatalf("Channel title = %q; want %q", rss.Channel.Title, "The Talk Show With John Gruber")
	}
	t.Logf("Parsed    https://daringfireball.net/thetalkshow/rss (%d items)", len(rss.Channel.Items))
	var extensionsMetadataCount int
	for _, item := range rss.Channel.Items {
		if len(item.Extensions) > 0 {
			extensionsMetadataCount++
			// for _, ext := range item.Extensions {
			// 	t.Logf("%s:%s", ext.XMLName.Space, ext.XMLName.Local)
			// }
		}
	}
	t.Logf("Extensions  https://daringfireball.net/thetalkshow/rss (%d properties)", extensionsMetadataCount)
	if len(rss.Channel.Items) == 0 {
		t.Fatalf("Expected items; got 0")
	}
}
