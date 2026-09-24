package feeder

import (
	"io"
	"strings"
	"testing"

	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/href"
)

func TestRSSParserDistinguishesAtomLinksAndEnclosures(t *testing.T) {
	feedURL, err := href.Parse("https://example.com/rss.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	parser := &RSSFeedParser{
		Bytes: []byte(`<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
	<channel>
		<title>Example</title>
		<link>https://example.com/</link>
		<description>Example feed</description>
		<item>
			<title>First</title>
			<enclosure url="https://example.com/posts/first/photo.png" type="image/png" length="5" />
			<atom:link rel="Enclosure" type="image/png" title="Photo" length="5" href="https://example.com/posts/first/photo.png" />
			<link>https://example.com/posts/first/</link>
			<atom:link rel="related" type="text/html" href="https://external.example/article" />
			<atom:link rel="CoMmEnTs" href="https://discussion.example/comments" />
			<atom:link rel="RePlIeS" href="https://discussion.example/replies" />
			<guid>first</guid>
			<description>Summary</description>
			<pubDate>Wed, 03 Jun 2026 00:00:00 GMT</pubDate>
		</item>
	</channel>
</rss>`),
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

	for _, attachment := range item.Attachments {
		if attachment.Href != nil && strings.Contains(attachment.Href.String(), "discussion.example") {
			t.Fatalf("discussion link became an attachment: %#v", attachment)
		}
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
		t.Fatalf("enclosure count = %d, want 1; attachments: %s", enclosureCount, attachmentURLs(item.Attachments))
	}
	if relatedCount != 1 {
		t.Fatalf("related attachment count = %d, want 1; attachments: %s", relatedCount, attachmentURLs(item.Attachments))
	}
	if duplicatedEnclosureLinks != 0 {
		t.Fatalf("enclosure duplicated as link attachment count = %d, want 0", duplicatedEnclosureLinks)
	}
}

func TestRSSParserUsesPermaLinkGUIDAfterAtomEnclosure(t *testing.T) {
	feedURL, err := href.Parse("https://example.com/rss.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	parser := &RSSFeedParser{
		Bytes: []byte(`<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
	<channel>
		<title>Example</title>
		<link>https://example.com/</link>
		<description>Example feed</description>
		<item>
			<title>First</title>
			<atom:link rel="enclosure" type="image/png" href="https://example.com/posts/first/photo.png" />
			<guid>first</guid>
			<description>Summary</description>
			<pubDate>Wed, 03 Jun 2026 00:00:00 GMT</pubDate>
		</item>
	</channel>
</rss>`),
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
	if item.URL == nil || item.URL.String() != "https://example.com/first" {
		t.Fatalf("item.URL = %v, want https://example.com/first", item.URL)
	}
	if len(item.Attachments) != 1 {
		t.Fatalf("attachments length = %d, want 1", len(item.Attachments))
	}
	if !item.Attachments[0].Enclosure {
		t.Fatalf("attachment.Enclosure = false, want true")
	}
}

func TestRSSParserUsesSourceURLAsLinkAttachment(t *testing.T) {
	feedURL, err := href.Parse("https://notes.example/feed.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	parser := &RSSFeedParser{
		Bytes: []byte(`<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
	<channel>
		<title>Example Notes</title>
		<link>https://notes.example/</link>
		<description>Example feed</description>
		<item>
			<title>First</title>
			<link>https://notes.example/n/1/</link>
			<source url="https://external.example/article"></source>
			<guid>first</guid>
			<description>Summary</description>
			<pubDate>Wed, 03 Jun 2026 00:00:00 GMT</pubDate>
		</item>
	</channel>
</rss>`),
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
	if got, want := item.URL.String(), "https://notes.example/n/1/"; got != want {
		t.Fatalf("item.URL = %q, want %q", got, want)
	}

	for _, attachment := range item.Attachments {
		if attachment.Href == nil || attachment.Href.String() != "https://external.example/article" {
			continue
		}
		if attachment.Enclosure {
			t.Fatalf("source attachment parsed as enclosure: %#v", attachment)
		}
		if attachment.Mimetype != "text/x-url" {
			t.Fatalf("source attachment mimetype = %q, want text/x-url", attachment.Mimetype)
		}
		return
	}
	t.Fatalf("source attachment not found; attachments: %s", attachmentURLs(item.Attachments))
}

// TestRSSParserToleratesMalformedItemContent ensures a single malformed link
// (in show-note HTML) or a single bad enclosure does not abort the whole feed:
// the item must still be parsed, with the bad pieces skipped.
func TestRSSParserToleratesMalformedItemContent(t *testing.T) {
	feedURL, err := href.Parse("https://example.com/rss.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	// content:encoded holds an <a href> whose value contains a newline (the
	// Daring Fireball "The Talk Show" case). The first enclosure has a
	// non-numeric length; the second has an unparseable url (invalid port).
	parser := &RSSFeedParser{
		Bytes: []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>" +
			"<rss version=\"2.0\" xmlns:content=\"http://purl.org/rss/1.0/modules/content/\">" +
			"<channel>" +
			"<title>Example</title>" +
			"<link>https://example.com/</link>" +
			"<description>Example feed</description>" +
			"<item>" +
			"<title>First</title>" +
			"<link>https://example.com/posts/first/</link>" +
			"<guid>first</guid>" +
			"<enclosure url=\"https://example.com/ep.mp3\" type=\"audio/mpeg\" length=\"notanumber\" />" +
			"<enclosure url=\"https://example.com:notaport/bad.mp3\" type=\"audio/mpeg\" length=\"5\" />" +
			"<content:encoded><![CDATA[<ul><li><a href=\"https://example.com/a\nb\">bad</a></li></ul>]]></content:encoded>" +
			"<pubDate>Wed, 03 Jun 2026 00:00:00 GMT</pubDate>" +
			"</item>" +
			"</channel>" +
			"</rss>"),
		Feed: &hyperfeed.Feed{URL: feedURL},
	}

	parsed, err := parser.Parse(io.Discard)
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil (malformed link/enclosure should be skipped)", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(parsed.Items))
	}
	item := parsed.Items[0]

	var foundGoodEnclosure bool
	for _, a := range item.Attachments {
		if a.Href == nil {
			t.Fatalf("attachment with nil href survived: %#v", a)
		}
		if a.Href.String() == "https://example.com/ep.mp3" {
			foundGoodEnclosure = true
			if a.Size != 0 {
				t.Fatalf("enclosure size = %d, want 0 (non-numeric length tolerated)", a.Size)
			}
		}
		if strings.Contains(a.Href.String(), "notaport") {
			t.Fatalf("enclosure with unparseable url should have been skipped: %s", a.Href.String())
		}
	}
	if !foundGoodEnclosure {
		t.Fatalf("valid enclosure was dropped; attachments: %s", attachmentURLs(item.Attachments))
	}
}

func attachmentURLs(attachments []hyperfeed.Attachment) string {
	urls := []string{}
	for _, attachment := range attachments {
		if attachment.Href != nil {
			urls = append(urls, attachment.Href.String())
		}
	}
	return strings.Join(urls, ",")
}

// This fixture follows https://news.ycombinator.com/rss: an external article
// link, a local comments URL, and no GUID.
func TestRSSParserPreservesHackerNewsArticleAttachment(t *testing.T) {
	feedURL, _ := href.Parse("https://news.ycombinator.com/rss")
	parser := &RSSFeedParser{
		Feed: &hyperfeed.Feed{URL: feedURL},
		Bytes: []byte(`<rss version="2.0"><channel>
			<title>Hacker News</title><link>https://news.ycombinator.com/</link>
			<description>Links for the intellectually curious, ranked by readers.</description>
			<item><title>Example article</title><link>https://external.example/article</link>
			<pubDate>Sat, 19 Sep 2026 10:46:58 +0000</pubDate>
			<comments>https://news.ycombinator.com/item?id=49765348</comments>
			<description><![CDATA[<a href="https://news.ycombinator.com/item?id=49765348">Comments</a>]]></description>
			</item></channel></rss>`),
	}
	feed, err := parser.Parse(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(feed.Items))
	}
	item := feed.Items[0]
	const discussionURL = "https://news.ycombinator.com/item?id=49765348"
	if item.URL == nil || item.URL.String() != discussionURL {
		t.Fatalf("item URL = %v, want %s", item.URL, discussionURL)
	}
	if item.CommentsURL == nil || item.CommentsURL.String() != discussionURL {
		t.Fatalf("comments URL = %v, want %s", item.CommentsURL, discussionURL)
	}
	if len(item.Attachments) != 1 {
		t.Fatalf("attachments = %#v, want one article link", item.Attachments)
	}
	attachment := item.Attachments[0]
	if attachment.Href == nil || attachment.Href.String() != "https://external.example/article" || attachment.Mimetype != "text/x-url" || attachment.Enclosure {
		t.Fatalf("article attachment = %#v", attachment)
	}
}
