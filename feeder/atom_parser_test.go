package feeder

import (
	"io"
	"testing"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	"github.com/herdworks/hyperfeed-go/types/href"
)

func TestAtomParserDistinguishesRelatedLinksAndEnclosures(t *testing.T) {
	feedURL, err := href.Parse("https://example.com/atom.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	parser := &AtomFeedParser{
		Bytes: []byte(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
	<title>Example</title>
	<link rel="alternate" type="text/html" href="https://example.com/" />
	<link rel="self" type="application/atom+xml" href="https://example.com/atom.xml" />
	<id>https://example.com/atom.xml</id>
	<updated>2026-06-03T00:00:00Z</updated>
	<entry>
		<title>First</title>
		<id>https://example.com/posts/first/</id>
		<link rel="Enclosure" type="image/png" title="Photo" length="5" href="https://example.com/posts/first/photo.png" />
		<link rel="alternate" type="text/html" href="https://example.com/posts/first/" />
		<link rel="related" type="text/html" href="https://external.example/article" />
		<published>2026-06-03T00:00:00Z</published>
		<updated>2026-06-03T00:00:00Z</updated>
		<author><name>Example Author</name><uri>https://example.com/</uri></author>
		<summary type="html">Summary</summary>
		<content type="html">Content</content>
	</entry>
</feed>`),
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
				t.Fatalf("related link parsed as enclosure: %#v", attachment)
			}
			relatedCount++
			if attachment.Mimetype != "text/x-url" {
				t.Fatalf("related attachment mimetype = %q, want text/x-url", attachment.Mimetype)
			}
		}
	}
	if enclosureCount != 1 {
		t.Fatalf("enclosure count = %d, want 1", enclosureCount)
	}
	if relatedCount != 1 {
		t.Fatalf("related attachment count = %d, want 1", relatedCount)
	}
	if duplicatedEnclosureLinks != 0 {
		t.Fatalf("enclosure duplicated as link attachment count = %d, want 0", duplicatedEnclosureLinks)
	}
}

func TestAtomParserDoesNotUseEnclosureAsItemURL(t *testing.T) {
	feedURL, err := href.Parse("https://example.com/atom.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	parser := &AtomFeedParser{
		Bytes: []byte(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
	<title>Example</title>
	<link rel="alternate" type="text/html" href="https://example.com/" />
	<id>https://example.com/atom.xml</id>
	<updated>2026-06-03T00:00:00Z</updated>
	<entry>
		<title>First</title>
		<id>https://example.com/posts/first/</id>
		<link rel="enclosure" type="image/png" href="https://example.com/posts/first/photo.png" />
		<published>2026-06-03T00:00:00Z</published>
		<updated>2026-06-03T00:00:00Z</updated>
		<summary type="html">Summary</summary>
	</entry>
</feed>`),
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
	if item.URL != nil {
		t.Fatalf("item.URL = %q, want nil", item.URL.String())
	}
	if len(item.Attachments) != 1 {
		t.Fatalf("attachments length = %d, want 1", len(item.Attachments))
	}
	if !item.Attachments[0].Enclosure {
		t.Fatalf("attachment.Enclosure = false, want true")
	}
}
