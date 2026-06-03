package feeder

import (
	"io"
	"testing"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	"github.com/herdworks/hyperfeed-go/types/href"
)

func TestJSONFeedParserDistinguishesExternalURLsAndAttachments(t *testing.T) {
	feedURL, err := href.Parse("https://example.com/feed.json")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	parser := &JSONFeedParser{
		Bytes: []byte(`{
	"version": "https://jsonfeed.org/version/1.1",
	"title": "Example",
	"home_page_url": "https://example.com/",
	"feed_url": "https://example.com/feed.json",
	"items": [
		{
			"id": "https://example.com/posts/first/",
			"url": "https://example.com/posts/first/",
			"external_url": "https://external.example/article",
			"title": "First",
			"summary": "Summary",
			"date_published": "2026-06-03T00:00:00Z",
			"attachments": [
				{"url": "https://example.com/posts/first/photo.png", "mime_type": "image/png", "title": "Photo", "size_in_bytes": 5}
			]
		}
	]
}`),
		Feed: &hyperfeed.Feed{URL: feedURL},
	}

	parsed, err := parser.Parse(io.Discard)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(parsed.Items))
	}
	item := parsed.Items[0]
	if got, want := item.URL.String(), "https://example.com/posts/first/"; got != want {
		t.Fatalf("item.URL = %q, want %q", got, want)
	}

	var enclosureCount, relatedCount, duplicatedEnclosureLinks int
	for _, attachment := range item.Attachments {
		if attachment.Href == nil {
			continue
		}
		switch attachment.Href.String() {
		case "https://example.com/posts/first/photo.png":
			if attachment.Enclosure {
				enclosureCount++
				if attachment.Mimetype != "image/png" || attachment.Title != "Photo" || attachment.Size != 5 {
					t.Fatalf("enclosure attachment = %#v, want image/png Photo size 5", attachment)
				}
			} else {
				duplicatedEnclosureLinks++
			}
		case "https://external.example/article":
			if attachment.Enclosure {
				t.Fatalf("external_url parsed as enclosure: %#v", attachment)
			}
			relatedCount++
			if attachment.Mimetype != "text/x-url" {
				t.Fatalf("external_url attachment mimetype = %q, want text/x-url", attachment.Mimetype)
			}
		}
	}
	if enclosureCount != 1 {
		t.Fatalf("enclosure count = %d, want 1; attachments: %s", enclosureCount, attachmentURLs(item.Attachments))
	}
	if relatedCount != 1 {
		t.Fatalf("external_url attachment count = %d, want 1; attachments: %s", relatedCount, attachmentURLs(item.Attachments))
	}
	if duplicatedEnclosureLinks != 0 {
		t.Fatalf("enclosure duplicated as link attachment count = %d, want 0", duplicatedEnclosureLinks)
	}
}
