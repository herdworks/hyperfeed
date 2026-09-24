package generator

import (
	"strings"
	"testing"
	"time"

	"github.com/herdworks/hq"
	"github.com/herdworks/hyperfeed"
	feedrss "github.com/herdworks/hyperfeed/rss"
)

func TestRSSGeneratorSerializesNormalizedFeed(t *testing.T) {
	feedURL := generatorHref(t, "https://example.com/feeds/podcast.xml")
	pageURL := generatorHref(t, "https://example.com/episodes/one/")
	coverURL := generatorHref(t, "https://cdn.example.com/cover.png")
	attachmentURL := generatorHref(t, "https://cdn.example.com/one.mp3")
	posterURL := generatorHref(t, "https://cdn.example.com/one.png")
	zero := int64(0)
	explicit := true
	feed := &hyperfeed.Feed{
		URL:         feedURL,
		HTMLURL:     generatorHref(t, "https://example.com/"),
		Title:       "Example Podcast",
		Description: "<p>Recent episodes</p>",
		Language:    "en-US",
		ModifiedAt:  time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
		IsPodcast:   true,
		Medium:      "podcast",
		CoverURL:    coverURL,
		Categories:  []hyperfeed.Category{hyperfeed.NewCategory("Open Source")},
		Bylines: []hyperfeed.Byline{
			{Name: "Second", Email: "second@example.com", Order: 2},
			{Name: "First", Email: "first@example.com", Order: 1},
		},
		Links: []*hyperfeed.Link{
			{Href: feedURL, Rel: "self", Mimetype: "application/rss+xml", Length: &zero},
			{Href: generatorHref(t, "https://example.com/"), Rel: "alternate", Mimetype: "text/html"},
		},
		Website: hyperfeed.Website{Entity: hq.Entity{FaviconURL: "https://example.com/favicon.ico"}},
		Items: []*hyperfeed.Item{{
			GUID:        pageURL.String(),
			URL:         pageURL,
			Title:       "Episode One",
			Summary:     "<p>Summary</p>",
			Content:     "<p>Content</p>",
			PublishedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
			CommentsURL: generatorHref(t, "https://example.com/episodes/one/#comments"),
			Categories:  []hyperfeed.Category{hyperfeed.NewCategory("Open Source")},
			Explicit:    &explicit,
			Bylines: []hyperfeed.Byline{
				{Name: "Second Creator", Role: "contributor", Order: 2},
				{Name: "First Creator", Role: "author", Order: 1},
			},
			Links: []*hyperfeed.Link{
				{Href: pageURL, Rel: "alternate", Mimetype: "text/html"},
				{Href: generatorHref(t, "mailto:comments@example.com"), Rel: "replies", Mimetype: "text/x-url"},
				{Href: generatorHref(t, "https://example.net/source"), Rel: "related", Mimetype: "text/html"},
			},
			Attachments: []hyperfeed.Attachment{{
				Href: attachmentURL, Poster: posterURL, Mimetype: "audio/mpeg", Enclosure: true, Size: 0, Duration: 65, Width: 640, Height: 360,
			}},
		}},
	}

	data, err := RSS(feed)
	if err != nil {
		t.Fatalf("RSS() error = %v", err)
	}
	output := string(data)
	for _, expected := range []string{
		`version="2.0"`,
		`xmlns:atom="http://www.w3.org/2005/Atom"`,
		`xmlns:content="http://purl.org/rss/1.0/modules/content/"`,
		`xmlns:dc="http://purl.org/dc/elements/1.1/"`,
		`xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`,
		`xmlns:media="http://search.yahoo.com/mrss/"`,
		`xmlns:podcast="https://podcastindex.org/namespace/1.0"`,
		`<link>https://example.com/</link>`,
		`<atom:link href="https://example.com/feeds/podcast.xml" rel="self" type="application/rss+xml" length="0"></atom:link>`,
		`<atom:link href="https://example.net/source" rel="related" type="text/html"></atom:link>`,
		`<managingEditor>first@example.com</managingEditor>`,
		`<lastBuildDate>Tue, 22 Sep 2026 12:00:00 +0000</lastBuildDate>`,
		`<category>Open Source</category>`,
		`<content:encoded>&lt;p&gt;Content&lt;/p&gt;</content:encoded>`,
		`<dc:creator>First Creator</dc:creator>`,
		`<dc:creator>Second Creator</dc:creator>`,
		`<guid>https://example.com/episodes/one/</guid>`,
		`<comments>https://example.com/episodes/one/#comments</comments>`,
		`<enclosure url="https://cdn.example.com/one.mp3" length="0" type="audio/mpeg"></enclosure>`,
		`<itunes:explicit>true</itunes:explicit>`,
		`<itunes:duration>65</itunes:duration>`,
		`<podcast:medium>podcast</podcast:medium>`,
		`<media:content url="https://cdn.example.com/one.mp3" type="audio/mpeg" duration="65" width="640" height="360"></media:content>`,
		`<media:thumbnail url="https://cdn.example.com/one.png" width="640" height="360"></media:thumbnail>`,
		`<media:keywords>Open Source</media:keywords>`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("generated RSS missing %s:\n%s", expected, output)
		}
	}
	if strings.Index(output, "<dc:creator>First Creator</dc:creator>") > strings.Index(output, "<dc:creator>Second Creator</dc:creator>") {
		t.Fatalf("creators are not ordered:\n%s", output)
	}
	for _, element := range []string{"<width>0</width>", "<height>0</height>", "<description></description>"} {
		if strings.Contains(output, element) {
			t.Fatalf("RSS image emitted an unspecified optional field %q:\n%s", element, output)
		}
	}
	for _, rel := range []string{"alternate", "replies"} {
		if strings.Contains(output, `rel="`+rel+`"`) {
			t.Fatalf("RSS emitted an Atom %s link:\n%s", rel, output)
		}
	}
	if _, err := feedrss.Parse(data); err != nil {
		t.Fatalf("generated RSS is not parseable: %v\n%s", err, data)
	}
}

func TestRSSGeneratorArtworkFallbackAndDerivedExplicitness(t *testing.T) {
	feed := &hyperfeed.Feed{
		URL:       generatorHref(t, "https://example.com/feed.xml"),
		Title:     "Example",
		Website:   hyperfeed.Website{Entity: hq.Entity{CoverURL: "https://example.com/site-cover.png", FaviconURL: "https://example.com/favicon.ico"}},
		IsPodcast: true,
		Items: []*hyperfeed.Item{{
			GUID:  "one",
			Title: "One",
		}},
	}
	data, err := RSS(feed)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	if !strings.Contains(output, `url="https://example.com/site-cover.png"`) || !strings.Contains(output, `<itunes:explicit>false</itunes:explicit>`) {
		t.Fatalf("website cover or false explicit missing:\n%s", output)
	}

	explicit := true
	feed.Items[0].Explicit = &explicit
	feed.Items[0].CoverURL = generatorHref(t, "https://example.com/item-cover.png")
	data, err = RSS(feed)
	if err != nil {
		t.Fatal(err)
	}
	output = string(data)
	if !strings.Contains(output, `<itunes:explicit>true</itunes:explicit>`) || !strings.Contains(output, `url="https://example.com/item-cover.png"`) {
		t.Fatalf("item cover or true explicit missing:\n%s", output)
	}
}

func TestRSSGeneratorUsesAlternateLinksAsCoreLinks(t *testing.T) {
	feed := &hyperfeed.Feed{
		URL:   generatorHref(t, "https://example.com/feed.xml"),
		Title: "Example",
		Links: []*hyperfeed.Link{{
			Href: generatorHref(t, "https://example.com/"), Rel: "alternate", Mimetype: "text/html",
		}},
		Items: []*hyperfeed.Item{{
			GUID: "entry",
			Links: []*hyperfeed.Link{{
				Href: generatorHref(t, "https://example.com/entry/"), Rel: "alternate", Mimetype: "text/html",
			}},
		}},
	}
	data, err := RSS(feed)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, `<link>https://example.com/</link>`) || !strings.Contains(got, `<link>https://example.com/entry/</link>`) || !strings.Contains(got, `<guid isPermaLink="false">entry</guid>`) {
		t.Fatalf("core links = %s", got)
	}
}

func TestRSSGeneratorDeclaresOnlyUsedNamespaces(t *testing.T) {
	feed := &hyperfeed.Feed{
		URL:         generatorHref(t, "https://example.com/feed.xml"),
		Title:       "Example",
		Description: "Description",
		Items: []*hyperfeed.Item{{
			GUID:  "one",
			Title: "One",
			URL:   generatorHref(t, "https://example.com/one/"),
		}},
	}
	data, err := RSS(feed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "xmlns:") {
		t.Fatalf("unused namespaces = %s", data)
	}
}

func TestRSSGeneratorRejectsInvalidInput(t *testing.T) {
	for name, feed := range map[string]*hyperfeed.Feed{
		"nil feed":    nil,
		"missing URL": {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RSS(feed); err == nil {
				t.Fatal("RSS() error = nil")
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
	if _, err := RSS(feed); err == nil || !strings.Contains(err.Error(), "negative size") {
		t.Fatalf("RSS() error = %v", err)
	}
}

var _ Generator = RSSGenerator{}
