package jsonfeed

import (
	"os"
	"testing"
)

// Sample minimal JSON Feed payload for unit tests.
const sampleJSONFeed = `{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "Sample Feed",
  "home_page_url": "https://example.com/",
  "feed_url": "/feed.json",
  "language": "en",
  "authors": [{"name": "Alice", "url": "/about"}],
  "items": [
    {
      "id": "1",
      "url": "/posts/1",
      "title": "Hello",
      "content_text": "Hello world",
      "date_published": "2024-01-02T03:04:05Z",
      "tags": ["intro"],
      "attachments": [
        {"url": "/audio/hello.mp3", "mime_type": "audio/mpeg", "title": "Hello MP3", "size_in_bytes": 123}
      ]
    },
    {
      "id": "2",
      "external_url": "https://example.org/article",
      "title": "Linkpost",
      "content_html": "<p>Check this out</p>",
      "date_modified": "2024-02-02T03:04:05Z"
    }
  ]
}`

// TestJSONFeedDetection tests the jsonfeed.Detected() function
func TestJSONFeedDetection(t *testing.T) {
	if !Detected([]byte(sampleJSONFeed)) {
		t.Fatalf("expected JSONFeedDetect to return true for valid JSON Feed")
	}
}

// TestJSONFeedParser tests the jsonfeed.Parse() function
func TestJSONFeedParser(t *testing.T) {
	body, err := os.ReadFile("testdata/daringfireball.net.jsonfeed.json")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	if !Detected(body) {
		t.Fatalf("expected JSONFeedDetect true for DF feed")
	}
	jf, err := Parse(body)
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if jf == nil || jf.Title == "" {
		t.Fatalf("expected non-empty feed title")
	}
	if len(jf.Items) == 0 {
		t.Fatalf("expected at least one item")
	}
	if jf.Items[0].ID == "" {
		t.Fatalf("expected first item to have a canonical id")
	}
	t.Logf("Daring Fireball has %d items", len(jf.Items))
	t.Logf("Daring Fireball has %d authors: %v", len(jf.Authors), jf.Authors)
}
