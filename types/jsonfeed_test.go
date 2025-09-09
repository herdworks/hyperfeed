package hyperfeed

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed-go/transporter"
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

func TestJSONFeedDetect_Valid(t *testing.T) {
	if !JSONFeedDetected([]byte(sampleJSONFeed)) {
		t.Fatalf("expected JSONFeedDetect to return true for valid JSON Feed")
	}
}

func TestJSONFeedParse_Minimal(t *testing.T) {
    transporter := transporter.NewTransporter()
	t.Logf("Fetching  https://daringfireball.net/feeds/json")
    body, err := transporter.Get("https://daringfireball.net/feeds/json")
	if err != nil {
		t.Skipf("fetch error: %v", err)
	}
	t.Logf("200 OK    https://daringfireball.net/feeds/json")
	jf, err := JSONFeedParse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Parsed    https://daringfireball.net/feeds/json")
	if jf == nil {
		t.Fatalf("expected feed, got nil")
	}
	if jf.Title != "Daring Fireball" {
		t.Fatalf("unexpected feed title: %q", jf.Title)
	}
	if jf.FeedURL == "" {
		t.Fatalf("expected resolved feed URL")
	}
	if len(jf.Items) == 0 {
		t.Fatalf("expected items, got %d", len(jf.Items))
	}
	t.Logf("Daring Fireball has %d items", len(jf.Items))
	t.Logf("The latest item is %q", jf.Items[0].Title)
    var externalURLs int
    for _, item := range jf.Items {
        if item.ExternalURL != "" {
            externalURLs++
        }
    }
	t.Logf("Items with external URLs: %d", externalURLs)
}

// TestJSONFeedIntegration_DaringFireball fetches and parses a live JSON Feed.
// Marked as integration via env gate to avoid network during default test runs.
func TestJSONFeedIntegration_DaringFireball(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") == "" {
		t.Skip("skipping integration test; set RUN_INTEGRATION=1 to enable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://daringfireball.net/feeds/json", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if !JSONFeedDetected(body) {
		t.Fatalf("expected JSONFeedDetect true for DF feed")
	}
	jf, err := JSONFeedParse(body)
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
}
