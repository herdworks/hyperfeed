package feeder

import (
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/herdworks/hq/username"
	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/atom"
	"github.com/herdworks/hyperfeed/href"
	"github.com/herdworks/hyperfeed/jsonfeed"
	"github.com/herdworks/hyperfeed/markdown"
	"github.com/herdworks/hyperfeed/rss"
	"github.com/herdworks/hyperfeed/xmlns"
)

// TYPES
// =============================================================================

type FeedParser interface {
	Ready() bool
	Parse(io.Writer) (*hyperfeed.Feed, error)
}

type RSSFeedParser struct {
	Bytes []byte
	RSS   *rss.RSS
	Feed  *hyperfeed.Feed
}

type AtomFeedParser struct {
	Bytes []byte
	Atom  *atom.Atom
	Feed  *hyperfeed.Feed
}

type JSONFeedParser struct {
	Bytes []byte
	JSON  *jsonfeed.Feed
	Feed  *hyperfeed.Feed
}

// NoFeedParser is a parser that handles empty feeds (e.g. zero byte responses on HTTP 304 Not Modified)
type NoFeedParser struct {
	Feed *hyperfeed.Feed
}

// VARIABLES
// =============================================================================

var (
	ErrUnsupportedFeedFormat = fmt.Errorf("unsupported feed format")
	ErrParserInitialization  = fmt.Errorf("feed parser initialization error")
	ErrParsingRawFeed        = fmt.Errorf("error parsing raw feed")
	ErrMissingHTMLURL        = fmt.Errorf("feed html_url missing")
	ErrInvalidRSSFeed        = fmt.Errorf("invalid RSS feed")
	ErrInvalidRSSItem        = fmt.Errorf("invalid RSS item(s)")
	ErrInvalidRSSEnclosure   = fmt.Errorf("invalid RSS enclosure(s)")
	ErrInvalidAtomFeed       = fmt.Errorf("invalid Atom feed")
	ErrInvalidAtomEntry      = fmt.Errorf("invalid Atom entry(ies)")
	ErrInvalidAtomLink       = fmt.Errorf("invalid Atom link(s)")
	ErrInvalidJSONFeed       = fmt.Errorf("invalid JSON Feed")
	ErrInvalidJSONItem       = fmt.Errorf("invalid JSON Feed item(s)")
	ErrInvalidJSONAttachment = fmt.Errorf("invalid JSON Feed attachment(s)")
)

// METHODS
// =============================================================================

// RSSFeedParser.Ready()
func (parser *RSSFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// RSSFeedParser.Parse()
func (parser *RSSFeedParser) Parse(w io.Writer) (*hyperfeed.Feed, error) {
	var err error
	if w == nil {
		w = os.Stdout
	}
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.RSS, err = rss.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}
	parser.RSS.Origin = parser.Feed.URL.URL()

	// Feed properties
	if link := parser.RSS.HTMLHref(); link != "" {
		if link == parser.Feed.URL.String() {
			link = href.ResolveHrefString(link, "/", w)
		}
		parser.Feed.HTMLURL, _ = href.Parse(link)
		parser.Feed.HTMLURL = href.ResolveHref(parser.Feed.URL, parser.Feed.HTMLURL, w) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.RSS.Channel.Title
	parser.Feed.Description, _ = markdown.Convert(parser.RSS.Channel.Description)
	parser.Feed.Language = parser.RSS.Channel.Language
	parser.Feed.Metadata = hyperfeed.Metadata{}
	parser.Feed.Email = parser.RSS.Channel.GetEmail()

	// Feed.Website properties
	if website := parser.Feed.WebsiteURL(); website == nil {
		fmt.Printf("FEED %q MISSING HTMLURL\n", parser.Feed.URL.String())
		return nil, errors.Join(ErrInvalidRSSFeed, ErrMissingHTMLURL)
	} else {
		parser.Feed.Website.HTMLURL = website.String()
		parser.Feed.Website.ID = parser.Feed.WebsiteIdentifier()
	}
	if parser.Feed.Website.FaviconURL == "" {
		// RSS does not have a standard favicon field, will source from HyperGraph later
	}
	if parser.Feed.Website.CoverURL == "" {
		// RSS does not have a standard cover field, will source from HyperGraph later
	}
	if parser.Feed.Website.MentionsURL == "" {
		// RSS does not have a standard mentions field, will source from HyperGraph later
	}
	if parser.Feed.Website.Email == "" {
		parser.Feed.Website.Email = parser.Feed.Email
	}
	if parser.Feed.Website.Name == "" {
		_, parser.Feed.Website.Name = rss.ParseAuthor(parser.RSS.Channel.ManagingEditor)
	}
	if parser.Feed.Website.Username == "" {
		parser.Feed.Website.Username = username.Parse(parser.Feed.HTMLURL.String())
	}
	if parser.Feed.Website.Description == "" {
		parser.Feed.Website.Description = parser.RSS.Channel.Description
	}

	// Feed.Items
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt, w); err != nil {
		return nil, err
	}
	if len(parser.RSS.Channel.Items) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.RSS.Channel.Items), len(parser.Feed.Items)))
	}
	return parser.Feed, nil
}

// RSSFeedParser.Items()
func (parser *RSSFeedParser) Items(added time.Time, w io.Writer) (items []*hyperfeed.Item, err error) {
	for index, rssitem := range parser.RSS.Channel.Items {
		item := &hyperfeed.Item{Feed: parser.Feed}
		item.GUID = rssitem.GUID.Value
		item.URL, _ = href.Parse(parser.RSS.ItemLink(&rssitem, parser.Feed.HTMLURL.URL()))
		item.Title = html.UnescapeString(strings.TrimSpace(rssitem.Title))
		if item.Summary, err = markdown.Convert(rssitem.Description); err != nil {
			fmt.Fprintf(w, "markdown RSS item.description error: %v\n", err)
		}
		if item.Summary == "" {
			if media := rssitem.Extensions.GetProperty(xmlns.YahooMediaRSSNS, "group"); media != nil {
				item.Summary = media.Extensions.GetPropertyString(xmlns.YahooMediaRSSNS, "description")
			}
		}
		if item.Content, err = markdown.Convert(rss.HTML(rssitem.ContentEncoded)); err != nil {
			fmt.Fprintf(w, "markdown RSS item.content:encoded error: %v\n", err)
		}
		item.CommentsURL, _ = href.Parse(rssitem.Comments.Value)
		item.PublishedAt, _ = rss.Date(rssitem.PubDate)
		item.ModifiedAt = item.PublishedAt // rss items don't have a modified date
		item.AddedAt = added
		item.UpdatedAt = added
		item.FeedAt = item.TimelineAt(-index) // use the index as a negative offset to preserve the original feed order of items
		if item.Bylines, err = parser.Bylines(&rssitem, item, w); err != nil {
			return nil, err
		}
		if item.Attachments, err = parser.Attachments(&rssitem, item, w); err != nil {
			return nil, err
		}
		if item.Links, err = hyperfeed.ParseLinks(item, []byte(hyperfeed.First(rssitem.ContentEncoded, rssitem.Description)), w); err != nil {
			return nil, err
		}
		item.ResolveRelativeURLs(w)
		item.DeduplicateAttachments()
		if item.ContainsPodcastMedia() {
			item.IsPodcastEpisode = true
			parser.Feed.IsPodcast = true
		}
		items = append(items, item)
	}
	return items, nil
}

// RSSFeedParser.Bylines()
func (parser *RSSFeedParser) Bylines(rssitem *rss.Item, item *hyperfeed.Item, w io.Writer) (bylines []hyperfeed.Byline, err error) {
	// Item dc:creator
	if byline := rssitem.Extensions.GetPropertyString(xmlns.DublicCoreMetadataNS, "creator"); byline != "" {
		name, email := rss.ParseAuthor(byline)
		bylines = append(bylines, hyperfeed.Byline{Name: strings.TrimSpace(name), Email: email})
	}
	return bylines, err
}

// RSSFeedParser.Attachments()
func (parser *RSSFeedParser) Attachments(rssitem *rss.Item, item *hyperfeed.Item, w io.Writer) (attachments []hyperfeed.Attachment, err error) {
	// attachment enclosures
	for _, enclosure := range rssitem.Enclosures {
		attachment := hyperfeed.Attachment{Sources: []hyperfeed.Source{}, Enclosure: true, Rank: len(attachments) + 1}
		// Skip (don't fail the whole feed for) an enclosure with an unparseable URL.
		if attachment.Href, _ = href.Parse(enclosure.URL); attachment.Href == nil {
			fmt.Fprintf(w, "skipping enclosure with unparseable url %q\n", enclosure.URL)
			continue
		}
		if enclosure.Length == "" {
			enclosure.Length = "0"
		}
		attachment.Size, _ = strconv.ParseInt(enclosure.Length, 10, 64) // tolerate a non-numeric length (defaults to 0)
		if duration := rss.GetPodcastEpisodeDurationSeconds(rssitem); duration > 0 {
			attachment.Duration = int64(duration)
		}
		if poster := rss.GetPodcastCoverArtwork(&parser.RSS.Channel, rssitem); poster != "" {
			attachment.Poster, _ = href.Parse(poster)
		}
		attachment.Mimetype = enclosure.Type
		attachments = append(attachments, attachment)
	}
	for _, link := range rssitem.Links {
		if !link.IsEnclosure() {
			continue
		}
		attachment := hyperfeed.Attachment{Sources: []hyperfeed.Source{}, Enclosure: true, Rank: len(attachments) + 1}
		if attachment.Href, _ = href.Parse(link.Href); attachment.Href == nil {
			fmt.Fprintf(w, "skipping enclosure link with unparseable href %q\n", link.Href)
			continue
		}
		attachment.Title = link.Title
		if link.Length != nil {
			attachment.Size = int64(*link.Length)
		}
		attachment.Mimetype = link.Type
		attachments = append(attachments, attachment)
	}

	// link attachments
	linkAttachments := hyperfeed.ParseItemLinkAttachments(item, rssitem.ExternalURLs(parser.Feed.HTMLURL.URL()), w)
	attachments = append(attachments, linkAttachments...)

	// inline media attachments
	mediaAttachments := hyperfeed.ParseItemMediaAttachments(item, []byte(hyperfeed.First(rssitem.ContentEncoded, rssitem.Description)), w)
	attachments = append(attachments, mediaAttachments...)

	// extension attachments
	extensionAttachments := hyperfeed.ParseExtensionAttachments(rssitem.Extensions)
	attachments = append(attachments, extensionAttachments...)

	for index := range attachments {
		if attachments[index].Mimetype == "" || attachments[index].Mimetype == "application/octet-stream" {
			attachments[index].Mimetype = hyperfeed.URLMimeType(attachments[index].Href.URL(), true)
		}
	}
	return attachments, nil
}

// AtomFeedParser.Ready()
func (parser *AtomFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// AtomFeedParser.Parse()
func (parser *AtomFeedParser) Parse(w io.Writer) (*hyperfeed.Feed, error) {
	var err error
	if w == nil {
		w = os.Stdout
	}
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.Atom, err = atom.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}
	parser.Atom.Origin = parser.Feed.URL.URL()

	// Feed properties
	if link := parser.Atom.Link("alternate"); link != nil {
		parser.Feed.HTMLURL, _ = href.Parse(link.Href)
		parser.Feed.HTMLURL = href.ResolveHref(parser.Feed.URL, parser.Feed.HTMLURL, w) // resolve relative URLs against the feed URL
	}
	if len(parser.Atom.Title) > 0 {
		parser.Feed.Title = parser.Atom.Title
	} else {
		for _, author := range parser.Atom.Authors {
			if author.Name != "" {
				parser.Feed.Title = author.Name
				break
			}
		}
	}
	parser.Feed.Description, _ = markdown.Convert(parser.Atom.Subtitle)
	parser.Feed.Language = "" // Atom does not have a standard language field
	parser.Feed.Metadata = hyperfeed.Metadata{}

	// Feed.Website properties
	if website := parser.Feed.WebsiteURL(); website == nil {
		fmt.Printf("FEED %q MISSING HTMLURL\n", parser.Feed.URL.String())
		return nil, errors.Join(ErrInvalidAtomFeed, ErrMissingHTMLURL)
	} else {
		parser.Feed.Website.HTMLURL = website.String()
		parser.Feed.Website.ID = parser.Feed.WebsiteIdentifier()
	}
	if parser.Feed.Website.FaviconURL == "" {
		if link := parser.Atom.Link("icon"); link != nil {
			parser.Feed.Website.FaviconURL = href.ResolveHrefString(parser.Feed.URL, link.Href, w)
		}
	}
	if parser.Feed.Website.CoverURL == "" {
		if link := parser.Atom.Link("cover"); link != nil {
			parser.Feed.Website.CoverURL = href.ResolveHrefString(parser.Feed.URL, link.Href, w)
		}
	}
	if parser.Feed.Website.MentionsURL == "" {
		if link := parser.Atom.Link("mentions"); link != nil {
			parser.Feed.Website.MentionsURL = href.ResolveHrefString(parser.Feed.URL, link.Href, w)
		}
	}
	if parser.Feed.Website.Email == "" {
		for _, author := range parser.Atom.Authors {
			if author.Email != "" {
				parser.Feed.Website.Email = author.Email
				parser.Feed.Email = author.Email
				break
			}
		}
	}
	if parser.Feed.Website.Name == "" {
		parser.Feed.Website.Name = parser.Atom.Title
	}
	if parser.Feed.Website.Username == "" {
		parser.Feed.Website.Username = username.Parse(parser.Feed.HTMLURL.String())
	}
	if parser.Feed.Website.Description == "" {
		parser.Feed.Website.Description = parser.Feed.Description
	}
	if !parser.Feed.Website.Verified {
		// placeholder
	}

	// Feed.Items
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt, w); err != nil {
		return nil, err
	}
	if len(parser.Atom.Entries) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.Atom.Entries), len(parser.Feed.Items)))
	}
	return parser.Feed, err
}

// AtomFeedParser.Items()
func (parser *AtomFeedParser) Items(added time.Time, w io.Writer) (items []*hyperfeed.Item, err error) {
	for index, entry := range parser.Atom.Entries {
		item := &hyperfeed.Item{Feed: parser.Feed}
		item.GUID = entry.ID
		if link := entry.InternalLink(parser.Feed.URL.URL()); link != nil {
			item.URL, _ = href.Parse(link.Href)
		}
		publishedTime, _ := time.Parse(time.RFC3339, entry.Published)
		modifiedTime, _ := time.Parse(time.RFC3339, entry.Updated)
		item.Title = html.UnescapeString(strings.TrimSpace(entry.Title))
		if item.Summary, err = markdown.Convert(entry.Summary.Body); err != nil {
			fmt.Fprintf(w, "markdown Atom entry.summary error: %v\n", err)
		}
		if item.Summary == "" {
			if media := entry.Extensions.GetProperty(xmlns.YahooMediaRSSNS, "group"); media != nil {
				item.Summary = media.Extensions.GetPropertyString(xmlns.YahooMediaRSSNS, "description")
			}
		}
		if item.Content, err = markdown.Convert(atom.HTML(entry.Content)); err != nil {
			fmt.Fprintf(w, "markdown Atom entry.content error: %v\n", err)
		}
		item.PublishedAt = hyperfeed.Earliest(publishedTime, modifiedTime)
		item.ModifiedAt = hyperfeed.Latest(modifiedTime, publishedTime)
		item.AddedAt = added
		item.UpdatedAt = added
		item.FeedAt = item.TimelineAt(-index) // use the index as a negative offset to preserve the original feed order of items
		item.CommentsURL = parser.Comments(entry, w)
		if item.Bylines, err = parser.Bylines(entry, item, w); err != nil {
			return nil, err
		}
		if item.Attachments, err = parser.Attachments(entry, item, w); err != nil {
			return nil, err
		}
		if item.Links, err = hyperfeed.ParseLinks(item, []byte(hyperfeed.First(entry.Content.Body, entry.Summary.Body)), w); err != nil {
			return nil, err
		}
		item.ResolveRelativeURLs(w)
		item.DeduplicateAttachments()
		if item.ContainsPodcastMedia() {
			item.IsPodcastEpisode = true
			parser.Feed.IsPodcast = true
		}
		items = append(items, item)
	}
	return items, nil
}

// Comments returns the first usable replies URL, falling back to comments.
func (parser *AtomFeedParser) Comments(entry *atom.Entry, w io.Writer) (commentsURL *href.Href) {
	for _, rel := range []string{"replies", "comments"} {
		for _, link := range entry.Links {
			if !strings.EqualFold(link.Rel, rel) || strings.TrimSpace(link.Href) == "" {
				continue
			}
			if commentsURL, _ = href.Parse(link.Href); commentsURL != nil {
				return commentsURL
			}
		}
	}
	return nil
}

// AtomFeedParser.Bylines()
func (parser *AtomFeedParser) Bylines(entry *atom.Entry, item *hyperfeed.Item, w io.Writer) (bylines []hyperfeed.Byline, err error) {
	for _, byline := range append(entry.Authors, entry.Contributors...) {
		href, _ := href.Parse(byline.URI)
		bylines = append(bylines, hyperfeed.Byline{Href: href, Name: strings.TrimSpace(byline.Name), Email: byline.Email})
	}
	return bylines, err
}

// AtomFeedParser.Attachments()
func (parser *AtomFeedParser) Attachments(entry *atom.Entry, item *hyperfeed.Item, w io.Writer) (attachments []hyperfeed.Attachment, err error) {
	// enclosure attachments
	for _, link := range entry.Links {
		attachment := hyperfeed.Attachment{Sources: []hyperfeed.Source{}, Enclosure: true, Rank: len(attachments) + 1}
		if !strings.EqualFold(link.Rel, "enclosure") {
			continue
		}
		attachment.Href, _ = href.Parse(link.Href)
		attachment.Title = link.Title
		if link.Length != nil {
			attachment.Size = int64(*link.Length)
		}
		attachment.Mimetype = link.Type
		attachment.Duration = 0 // Atom links do not have a duration field
		attachment.Mimetype = link.Type
		attachments = append(attachments, attachment)
	}

	// link attachments
	linkAttachments := hyperfeed.ParseItemLinkAttachments(item, entry.ExternalURLs(parser.Feed.HTMLURL.URL()), w)
	attachments = append(attachments, linkAttachments...)

	// inline media attachments
	mediaAttachments := hyperfeed.ParseItemMediaAttachments(item, []byte(hyperfeed.First(entry.Content.Body, entry.Summary.Body)), w)
	attachments = append(attachments, mediaAttachments...)

	// extension attachments
	extensionAttachments := hyperfeed.ParseExtensionAttachments(entry.Extensions)
	attachments = append(attachments, extensionAttachments...)

	for index := range attachments {
		if attachments[index].Mimetype == "" || attachments[index].Mimetype == "application/octet-stream" {
			attachments[index].Mimetype = hyperfeed.URLMimeType(attachments[index].Href.URL(), true)
		}
	}
	return attachments, nil
}

// JSONFeedParser.Ready()
func (parser *JSONFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// JSONFeedParser.Parse()
func (parser *JSONFeedParser) Parse(w io.Writer) (*hyperfeed.Feed, error) {
	var err error
	if w == nil {
		w = os.Stdout
	}
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.JSON, err = jsonfeed.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}
	parser.JSON.Origin = parser.Feed.URL.URL()

	// Feed properties
	if link := parser.JSON.HomePageURL; link != "" {
		parser.Feed.HTMLURL, _ = href.Parse(link)
		parser.Feed.HTMLURL = href.ResolveHref(parser.Feed.URL, parser.Feed.HTMLURL, w) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.JSON.Title
	parser.Feed.Description, _ = markdown.Convert(parser.JSON.Description)
	parser.Feed.Language = parser.JSON.Language
	parser.Feed.Metadata = hyperfeed.Metadata{}

	// Feed.Website properties
	if website := parser.Feed.WebsiteURL(); website == nil {
		fmt.Printf("FEED %q MISSING HTMLURL\n", parser.Feed.URL.String())
		return nil, errors.Join(ErrInvalidJSONFeed, ErrMissingHTMLURL)
	} else {
		parser.Feed.Website.HTMLURL = website.String()
		parser.Feed.Website.ID = parser.Feed.WebsiteIdentifier()
	}
	if parser.Feed.Website.FaviconURL == "" {
		parser.Feed.Website.FaviconURL = href.ResolveHrefString(parser.Feed.URL, hyperfeed.First(parser.JSON.Favicon, parser.JSON.Icon), w)
	}
	if parser.Feed.Website.CoverURL == "" {
		parser.Feed.Website.CoverURL = href.ResolveHrefString(parser.Feed.URL, parser.JSON.Icon, w)
	}
	if parser.Feed.Website.MentionsURL == "" {
		// JSON Feed does not have a standard mentions field, will source from HyperGraph later
	}
	if parser.Feed.Website.Email == "" {
		// JSON Feed does not have a standard email field, will source from HyperGraph later
	}
	if parser.Feed.Website.Name == "" {
		parser.Feed.Website.Name = parser.JSON.Title
	}
	if parser.Feed.Website.Username == "" {
		parser.Feed.Website.Username = username.Parse(parser.Feed.HTMLURL.String())
	}
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt, w); err != nil {
		return nil, err
	}
	if len(parser.JSON.Items) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.JSON.Items), len(parser.Feed.Items)))
	}
	return parser.Feed, nil
}

// JSONFeedParser.Items()
func (parser *JSONFeedParser) Items(added time.Time, w io.Writer) (items []*hyperfeed.Item, err error) {
	for index, jsonitem := range parser.JSON.Items {
		item := &hyperfeed.Item{Feed: parser.Feed}
		item.URL, _ = href.Parse(hyperfeed.First(jsonitem.URL, jsonitem.ID)) // prefer URL, fallback to ID
		publishedTime, _ := time.Parse(time.RFC3339, jsonitem.DatePublished)
		modifiedTime, _ := time.Parse(time.RFC3339, jsonitem.DateModified)
		item.Title = html.UnescapeString(strings.TrimSpace(jsonitem.Title))
		if item.Summary, err = markdown.Convert(jsonitem.Summary); err != nil {
			fmt.Fprintf(w, "markdown JSONFeed item.Summary error: %v\n", err)
		}
		if item.Content, err = markdown.Convert(jsonfeed.HTML(hyperfeed.First(jsonitem.ContentHTML, jsonitem.ContentText))); err != nil {
			fmt.Fprintf(w, "markdown JSONFeed item.Content error: %v\n", err)
		}
		item.CommentsURL = nil // JSONFeed items do not have a comments field
		item.PublishedAt = hyperfeed.Earliest(publishedTime, modifiedTime)
		item.ModifiedAt = hyperfeed.Latest(modifiedTime, publishedTime)
		item.AddedAt = added
		item.UpdatedAt = added
		item.FeedAt = item.TimelineAt(-index) // use the index as a negative offset to preserve the original feed order of items
		if item.Bylines, err = parser.Bylines(&jsonitem, item, w); err != nil {
			return nil, err
		}
		if item.Attachments, err = parser.Attachments(&jsonitem, item, w); err != nil {
			return nil, err
		}
		if item.Links, err = hyperfeed.ParseLinks(item, []byte(jsonfeed.HTML(hyperfeed.First(jsonitem.ContentHTML, jsonitem.ContentText, jsonitem.Summary))), w); err != nil {
			return nil, err
		}
		item.ResolveRelativeURLs(w)
		item.DeduplicateAttachments()
		if item.ContainsPodcastMedia() {
			item.IsPodcastEpisode = true
			parser.Feed.IsPodcast = true
		}
		items = append(items, item)
	}
	return items, nil
}

// JSONFeedParser.Bylines()
func (parser *JSONFeedParser) Bylines(jsonfeeditem *jsonfeed.Item, item *hyperfeed.Item, w io.Writer) (bylines []hyperfeed.Byline, err error) {
	for _, byline := range jsonfeeditem.Authors {
		url, _ := href.Parse(byline.URL)
		favicon, _ := href.Parse(byline.Avatar)
		bylines = append(bylines, hyperfeed.Byline{Href: url, Name: strings.TrimSpace(byline.Name), Favicon: favicon})
	}
	return bylines, err
}

// JSONFeedParser.Attachments()
func (parser *JSONFeedParser) Attachments(jsonfeeditem *jsonfeed.Item, item *hyperfeed.Item, w io.Writer) (attachments []hyperfeed.Attachment, err error) {
	for _, a := range jsonfeeditem.Attachments {
		attachment := hyperfeed.Attachment{Sources: []hyperfeed.Source{}, Enclosure: true, Rank: len(attachments) + 1}
		attachment.Href, _ = href.Parse(a.URL)
		attachment.Title = a.Title
		attachment.Size = int64(a.SizeInBytes)
		attachment.Mimetype = a.MimeType
		attachment.Duration = a.DurationInSeconds
		attachments = append(attachments, attachment)
	}
	linkAttachments := hyperfeed.ParseItemLinkAttachments(item, jsonfeeditem.ExternalURLs(parser.Feed.HTMLURL.URL()), w)
	attachments = append(attachments, linkAttachments...)
	mediaAttachments := hyperfeed.ParseItemMediaAttachments(item, []byte(jsonfeed.HTML(hyperfeed.First(jsonfeeditem.ContentHTML, jsonfeeditem.ContentText, jsonfeeditem.Summary))), w)
	attachments = append(attachments, mediaAttachments...)
	for index := range attachments {
		if attachments[index].Mimetype == "" || attachments[index].Mimetype == "application/octet-stream" {
			attachments[index].Mimetype = hyperfeed.URLMimeType(attachments[index].Href.URL(), true)
		}
	}
	return attachments, nil
}

// NoFeedParser.Ready()
func (parser *NoFeedParser) Ready() bool {
	return true
}

// NoFeedParser.Parse()
func (parser *NoFeedParser) Parse(_ io.Writer) (*hyperfeed.Feed, error) {
	return parser.Feed, nil
}

// FUNCTIONS
// =============================================================================

// NewFeedParser detects and returns an appropriate FeedParserKind for the given feed bytes.
func NewFeedParser(feed *hyperfeed.Feed, bytes []byte) (parser FeedParser, err error) {
	if bytes == nil || feed == nil || feed.URL == nil {
		return nil, errors.Join(ErrParserInitialization, errors.New("nil feed passed to NewFeedParser()"))
	}
	switch {
	case len(bytes) == 0:
		// empty body (e.g. HTTP 304 Not Modified); preserve any previously detected kind
		if feed.Kind == "" {
			feed.Kind = "unknown"
		}
		parser = &NoFeedParser{Feed: feed}
	case rss.Detected(bytes):
		feed.Kind = "rss"
		parser = &RSSFeedParser{Bytes: bytes, Feed: feed}
	case atom.Detected(bytes):
		feed.Kind = "atom"
		parser = &AtomFeedParser{Bytes: bytes, Feed: feed}
	case jsonfeed.Detected(bytes):
		feed.Kind = "jsonfeed"
		parser = &JSONFeedParser{Bytes: bytes, Feed: feed}
	}
	return parser, nil
}

func FeedKind(bytes []byte) (kind string) {
	switch {
	case rss.Detected(bytes):
		kind = "rss"
	case atom.Detected(bytes):
		kind = "atom"
	case jsonfeed.Detected(bytes):
		kind = "jsonfeed"
	default:
		kind = "unknown"
	}
	return kind
}
