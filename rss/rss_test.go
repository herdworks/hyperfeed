package rss

import (
	"encoding/xml"
	"net/url"
	"os"
	"strings"
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
	if !Detected([]byte(sampleRSS)) {
		t.Fatalf("RSSDetect returned false")
	}
}

func TestRSSParse(t *testing.T) {
	rss, err := Parse([]byte(sampleRSS))
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

func TestRSSParseAcceptsAlternateRoot(t *testing.T) {
	feed, err := Parse([]byte(`<not-rss><channel><title>Example</title><link>https://example.com/</link><description>Sample</description></channel></not-rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if feed.XMLName.Local != "not-rss" || feed.Channel.Title != "Example" {
		t.Fatalf("parsed feed = %#v", feed)
	}
}

func TestRSSGUIDPermaLinkDefaultsAndSerialization(t *testing.T) {
	origin, err := url.Parse("https://example.com/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	for name, document := range map[string]string{
		"absent": `<rss><channel><item><guid>https://example.com/one</guid></item></channel></rss>`,
		"true":   `<rss><channel><item><guid isPermaLink="true">https://example.com/one</guid></item></channel></rss>`,
		"false":  `<rss><channel><item><guid isPermaLink="false">https://example.com/one</guid></item></channel></rss>`,
	} {
		t.Run(name, func(t *testing.T) {
			feed, err := Parse([]byte(document))
			if err != nil {
				t.Fatal(err)
			}
			item := feed.Channel.Items[0]
			if got, want := item.GUID.IsPermaLink, name != "false"; got != want {
				t.Fatalf("isPermaLink = %t, want %t", got, want)
			}
			if name != "false" && feed.ItemLink(&item, origin) == nil {
				t.Fatal("permalink GUID did not resolve to an item link")
			}
		})
	}
	feed := RSS{
		XMLName: xml.Name{Local: "rss"},
		Version: "2.0",
		Channel: Channel{Items: []Item{{
			GUID: RSSGUID{Value: "https://example.com/one", IsPermaLink: true},
		}, {
			GUID: RSSGUID{Value: "opaque", IsPermaLink: false},
		}}},
	}
	data, err := xml.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	if !strings.Contains(output, `<guid>https://example.com/one</guid>`) || !strings.Contains(output, `<guid isPermaLink="false">opaque</guid>`) {
		t.Fatalf("GUID XML = %s", output)
	}
}

func TestRSSParseTheTalkShow(t *testing.T) {
	body, err := os.ReadFile("testdata/daringfireball.net.thetalkshow.rss.xml")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	rss, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if rss.Channel.Title == "" {
		t.Fatalf("Channel title is empty")
	}
	t.Logf("The Talk Show has %d items", len(rss.Channel.Items))
	if len(rss.Channel.Items) == 0 {
		t.Fatalf("Expected items; got 0")
	}
}

func TestRSSAuthorNameParser(t *testing.T) {
	authorTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "name only",
			input:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "name and email",
			input:    "Jane Smith <jane@example.com>",
			expected: "Jane Smith",
		},
		{
			name:     "email first",
			input:    "jane@example.com <Jane Smith>",
			expected: "Jane Smith",
		},
		{
			name:     "name with parentheses email",
			input:    "John Doe (john@example.com)",
			expected: "John Doe",
		},
		{
			name:     "email only",
			input:    "jane@example.com",
			expected: "",
		},
	}
	for _, tc := range authorTests {
		t.Run(tc.name, func(t *testing.T) {
			name, _ := ParseAuthor(tc.input)
			if name != tc.expected {
				t.Errorf("ParseAuthorName(%q) = %q; want %q", tc.input, name, tc.expected)
			}

		})
	}
}

func TestRSSAuthorEmailParser(t *testing.T) {
	authorTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "name only",
			input:    "John Doe",
			expected: "",
		},
		{
			name:     "name and email",
			input:    "Jane Smith <jane@example.com>",
			expected: "jane@example.com",
		},
		{
			name:     "email first",
			input:    "jane@example.com <Jane Smith>",
			expected: "jane@example.com",
		},
		{
			name:     "name with parentheses email",
			input:    "John Doe (john@example.com)",
			expected: "john@example.com",
		},
		{
			name:     "email only",
			input:    "jane@example.com",
			expected: "jane@example.com",
		},
	}
	for _, tc := range authorTests {
		t.Run(tc.name, func(t *testing.T) {
			_, email := ParseAuthor(tc.input)
			if email != tc.expected {
				t.Errorf("ParseAuthorEmail(%q) = %q; want %q", tc.input, email, tc.expected)
			}

		})
	}
}
