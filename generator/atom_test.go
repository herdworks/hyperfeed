package generator

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/href"
)

func TestAtomGeneratorSerializesNormalizedFeed(t *testing.T) {
	feedURL := generatorHref(t, "https://example.com/feeds/blog.xml")
	pageURL := generatorHref(t, "https://example.com/posts/one/")
	authorURL := generatorHref(t, "https://example.com/authors/first")
	attachmentURL := generatorHref(t, "https://cdn.example.com/one.mp3")
	zero := int64(0)
	feed := &hyperfeed.Feed{
		URL:         feedURL,
		Title:       "Blog",
		Description: "Recent writing",
		ModifiedAt:  time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
		Links: []*hyperfeed.Link{
			{Href: feedURL, Rel: "self", Mimetype: "application/atom+xml", Length: &zero},
			{Href: generatorHref(t, "https://example.com/"), Rel: "alternate", Mimetype: "text/html"},
		},
		Bylines: []hyperfeed.Byline{
			{Name: "Second", Role: "author", Order: 2},
			{Name: "First", Href: authorURL, Favicon: generatorHref(t, "https://example.com/authors/first.png"), Role: "author", Order: 1},
			{Name: "Unspecified", Role: "author"},
			{Name: "Editor", Role: "contributor", Order: 1},
		},
		Items: []*hyperfeed.Item{{
			GUID:        pageURL.String(),
			Title:       "One",
			PublishedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
			ModifiedAt:  time.Date(2026, 9, 22, 11, 0, 0, 0, time.UTC),
			Summary:     "<p>Summary</p>",
			Content:     "<p>Content</p>",
			Links:       []*hyperfeed.Link{{Href: pageURL, Rel: "alternate", Mimetype: "text/html"}},
			Bylines: []hyperfeed.Byline{
				{Name: "Writer", Role: "author", Order: 1},
				{Name: "Editor", Role: "contributor", Order: 2},
			},
			Attachments: []hyperfeed.Attachment{{
				Href: attachmentURL, Mimetype: "audio/mpeg", Title: "Episode", Enclosure: true, Size: 42,
			}},
		}},
	}

	data, err := Atom(feed)
	if err != nil {
		t.Fatalf("Atom() error = %v", err)
	}
	var decoded struct {
		ID      string `xml:"id"`
		Updated string `xml:"updated"`
		Authors []struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Contributors []struct {
			Name string `xml:"name"`
		} `xml:"contributor"`
		Entries []struct {
			ID      string `xml:"id"`
			Summary struct {
				Body string `xml:",chardata"`
			} `xml:"summary"`
			Content struct {
				Body string `xml:",chardata"`
			} `xml:"content"`
			Contributors []struct {
				Name string `xml:"name"`
			} `xml:"contributor"`
			Links []struct {
				Rel    string `xml:"rel,attr"`
				Href   string `xml:"href,attr"`
				Type   string `xml:"type,attr"`
				Length uint   `xml:"length,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("generated XML: %v\n%s", err, data)
	}
	if !strings.Contains(string(data), `length="0"`) {
		t.Fatalf("generated XML omitted explicit zero length:\n%s", data)
	}
	if strings.Contains(string(data), "hypergraph:") || strings.Contains(string(data), "xmlns:hypergraph") {
		t.Fatalf("generated XML includes a HyperGraph extension:\n%s", data)
	}
	if decoded.ID != feedURL.String() || decoded.Updated != "2026-09-22T12:00:00Z" {
		t.Fatalf("feed identity = %#v", decoded)
	}
	if got, want := bylineNames(decoded.Authors), "First,Second,Unspecified"; got != want {
		t.Fatalf("feed authors = %q, want %q", got, want)
	}
	if got, want := bylineNames(decoded.Contributors), "Editor"; got != want {
		t.Fatalf("feed contributors = %q, want %q", got, want)
	}
	if len(decoded.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(decoded.Entries))
	}
	entry := decoded.Entries[0]
	if entry.ID != pageURL.String() || entry.Summary.Body != "<p>Summary</p>" || entry.Content.Body != "<p>Content</p>" {
		t.Fatalf("entry = %#v", entry)
	}
	if got, want := bylineNames(entry.Contributors), "Editor"; got != want {
		t.Fatalf("entry contributors = %q, want %q", got, want)
	}
	if len(entry.Links) != 2 || entry.Links[1].Rel != "enclosure" || entry.Links[1].Href != attachmentURL.String() || entry.Links[1].Type != "audio/mpeg" || entry.Links[1].Length != 42 {
		t.Fatalf("entry links = %#v", entry.Links)
	}
}

func TestAtomGeneratorRejectsInvalidInput(t *testing.T) {
	for name, feed := range map[string]*hyperfeed.Feed{
		"nil feed":    nil,
		"missing URL": {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Atom(feed); err == nil {
				t.Fatal("Atom() error = nil")
			}
		})
	}
	feed := &hyperfeed.Feed{
		URL: generatorHref(t, "https://example.com/feed.xml"),
		Items: []*hyperfeed.Item{{
			Attachments: []hyperfeed.Attachment{{
				Href: generatorHref(t, "https://example.com/negative.bin"), Enclosure: true, Size: -1,
			}},
		}},
	}
	if _, err := Atom(feed); err == nil || !strings.Contains(err.Error(), "negative size") {
		t.Fatalf("Atom() error = %v", err)
	}
}

func generatorHref(t *testing.T, raw string) *href.Href {
	t.Helper()
	result, err := href.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func bylineNames(values []struct {
	Name string `xml:"name"`
}) string {
	names := make([]string, 0, len(values))
	for _, value := range values {
		names = append(names, value.Name)
	}
	return strings.Join(names, ",")
}

var _ Generator = AtomGenerator{}
