package feeder

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	atom "github.com/herdworks/hyperfeed-go/types/atom"
	href "github.com/herdworks/hyperfeed-go/types/href"
	jsonfeed "github.com/herdworks/hyperfeed-go/types/jsonfeed"
	markdown "github.com/herdworks/hyperfeed-go/types/markdown"
	rss "github.com/herdworks/hyperfeed-go/types/rss"
	username "github.com/herdworks/hypergraph-go/username"
)

// TYPES
// =============================================================================

type FeedParser interface {
	Ready() bool
	Parse(time.Time) (*hyperfeed.Feed, error)
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
func (parser *RSSFeedParser) Parse(timelinePosition time.Time) (*hyperfeed.Feed, error) {
	var err error
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.RSS, err = rss.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}

	// Feed properties
	if parser.Feed.HTMLURL == nil || parser.Feed.URL.String() == parser.Feed.HTMLURL.String() {
		parser.Feed.HTMLURL, _ = href.Parse(parser.RSS.Link())
		parser.Feed.HTMLURL = parser.Feed.URL.ResolveReference(parser.Feed.HTMLURL.URL()) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.RSS.Channel.Title
	parser.Feed.Description = parser.RSS.Channel.Description
	parser.Feed.Language = parser.RSS.Channel.Language
	parser.Feed.Metadata = hyperfeed.Metadata{}

	// Feed.Website properties
	if parser.Feed.Website.HTMLURL == "" {
		parser.Feed.Website.HTMLURL = parser.Feed.HTMLURL.String()
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
		parser.Feed.Website.Email, _ = rss.ParseAuthor(parser.RSS.Channel.ManagingEditor)
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
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt, timelinePosition); err != nil {
		return nil, err
	}
	if len(parser.RSS.Channel.Items) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.RSS.Channel.Items), len(parser.Feed.Items)))
	}
	return parser.Feed, nil
}

// RSSFeedParser.Items()
func (parser *RSSFeedParser) Items(added time.Time, timelinePosition time.Time) (items []*hyperfeed.Item, err error) {
	for _, rssitem := range parser.RSS.Channel.Items {
		item := &hyperfeed.Item{Feed: parser.Feed}
		if item.URL = rss.LinkURL(rssitem.Links); item.URL != nil && !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		if !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		item.Title = rssitem.Title
		item.Images = hyperfeed.ParseImages(item, []byte(hyperfeed.First(rssitem.ContentEncoded, rssitem.Description)))
		if item.Summary, err = markdown.Convert(hyperfeed.First(rssitem.Description, rssitem.ContentEncoded)); err != nil {
			fmt.Printf("markdown RSS item.description error: %v\n", err)
		}
		if item.Content, err = markdown.Convert(rss.HTML(hyperfeed.First(rssitem.ContentEncoded, rssitem.Description))); err != nil {
			fmt.Printf("markdown RSS item.content:encoded error: %v\n", err)
		}
		item.Comments, _ = url.Parse(rssitem.Comments)
		item.PublishedAt, _ = rss.Date(rssitem.PubDate)
		item.ModifiedAt = item.PublishedAt // rss items don't have a modified date
		item.AddedAt = added
		item.UpdatedAt = added
		item.FeedAt = item.TimelineAt(timelinePosition)
		if byline := rss.GetDublinCoreCreator(rssitem.Extensions); byline != "" {
			name, email := rss.ParseAuthor(byline)
			item.Bylines = append(item.Bylines, hyperfeed.Byline{Name: name, Email: email})
		}
		if item.Attachments, err = parser.Attachments(&rssitem, item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// RSSFeedParser.Attachments()
func (parser *RSSFeedParser) Attachments(rssitem *rss.Item, item *hyperfeed.Item) (attachments []hyperfeed.Attachment, err error) {
	for _, enclosure := range rssitem.Enclosures {
		attachment := hyperfeed.Attachment{}
		if attachment.URL, err = href.Parse(enclosure.URL); err != nil {
			return nil, err
		}
		if enclosure.Length == "" {
			enclosure.Length = "0"
		}
		if attachment.Size, err = strconv.ParseInt(enclosure.Length, 10, 64); err != nil {
			return nil, err
		}
		if duration := rss.GetItunesMetadata(rssitem.Extensions, "duration"); duration != nil {
			if v, err := time.Parse("15:04:05", duration.Text); err == nil {
				attachment.Duration = int64(v.Hour()*3600 + v.Minute()*60 + v.Second()) // duration in seconds
			}
		}
		attachment.Mimetype = enclosure.Type
		attachment.ItemID = item.Identifier()
		attachment.FeedID = item.Feed.Identifier()
		attachment.WebsiteID = item.Feed.Website.Identifier()
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// AtomFeedParser.Ready()
func (parser *AtomFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// AtomFeedParser.Parse()
func (parser *AtomFeedParser) Parse(timelinePosition time.Time) (*hyperfeed.Feed, error) {
	var err error
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.Atom, err = atom.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}

	// Feed properties
	if parser.Feed.HTMLURL == nil || parser.Feed.URL.String() == parser.Feed.HTMLURL.String() {
		if link := parser.Atom.Link("alternate"); link != nil {
			parser.Feed.HTMLURL, _ = href.Parse(link.Href)
			parser.Feed.HTMLURL = parser.Feed.URL.ResolveReference(parser.Feed.HTMLURL.URL()) // resolve relative URLs against the feed URL
		}
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
	parser.Feed.Description = parser.Atom.Subtitle
	parser.Feed.Language = "" // Atom does not have a standard language field
	parser.Feed.Metadata = hyperfeed.Metadata{}

	// Feed.Website properties
	if parser.Feed.Website.FaviconURL == "" {
		if link := parser.Atom.Link("icon"); link != nil {
			parser.Feed.Website.FaviconURL = parser.Feed.HTMLURL.String()
		}
	}
	if parser.Feed.Website.CoverURL == "" {
		if link := parser.Atom.Link("cover"); link != nil {
			parser.Feed.Website.CoverURL = href.ResolveHrefString(parser.Feed.URL, link.Href)
		}
	}
	if parser.Feed.Website.MentionsURL == "" {
		if link := parser.Atom.Link("mentions"); link != nil {
			parser.Feed.Website.MentionsURL = href.ResolveHrefString(parser.Feed.URL, link.Href)
		}
	}
	if parser.Feed.Website.Email == "" {
		for _, author := range parser.Atom.Authors {
			if author.Email != "" {
				parser.Feed.Website.Email = author.Email
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
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt, timelinePosition); err != nil {
		return nil, err
	}
	if len(parser.Atom.Entries) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.Atom.Entries), len(parser.Feed.Items)))
	}
	return parser.Feed, err
}

// AtomFeedParser.Items()
func (parser *AtomFeedParser) Items(added time.Time, timelinePosition time.Time) (items []*hyperfeed.Item, err error) {
	for _, entry := range parser.Atom.Entries {
		item := &hyperfeed.Item{Feed: parser.Feed}
		if link := entry.InternalLink(parser.Feed.URL.URL()); link != nil {
			item.URL, _ = url.Parse(link.Href)
		}
		if links := entry.ExternalLinks(parser.Feed.URL.URL()); len(links) > 0 {
			for _, l := range links {
				item.ExternalHrefs = append(item.ExternalHrefs, l.Href)
			}
		}
		if !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		publishedTime, _ := time.Parse(time.RFC3339, entry.Published)
		modifiedTime, _ := time.Parse(time.RFC3339, entry.Updated)
		summary := atom.First(&atom.Text{}, entry.Summary, entry.Content)
		content := atom.First(&atom.Text{}, entry.Content, entry.Summary)
		item.Title = entry.Title
		item.Images = hyperfeed.ParseImages(item, []byte(hyperfeed.First(content.Body, summary.Body)))
		if item.Summary, err = markdown.Convert(summary.Body); err != nil {
			fmt.Printf("markdown Atem entry.summary error: %v\n", err)
		}
		if item.Content, err = markdown.Convert(atom.HTML(atom.First(entry.Content, entry.Summary))); err != nil {
			fmt.Printf("markdown Atom entry.content error: %v\n", err)
		}
		item.Comments = nil // Atom entries do not have a comments field
		item.PublishedAt = hyperfeed.Earliest(publishedTime, modifiedTime)
		item.ModifiedAt = hyperfeed.Latest(modifiedTime, publishedTime)
		item.AddedAt = added
		item.UpdatedAt = added
		item.FeedAt = item.TimelineAt(timelinePosition)
		for _, byline := range append(entry.Authors, entry.Contributors...) {
			href, _ := href.Parse(byline.URI)
			item.Bylines = append(item.Bylines, hyperfeed.Byline{Href: href, Name: byline.Name, Email: byline.Email})
		}
		if item.Attachments, err = parser.Attachments(entry, item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// AtomFeedParser.Attachments()
func (parser *AtomFeedParser) Attachments(entry *atom.Entry, item *hyperfeed.Item) (attachments []hyperfeed.Attachment, err error) {
	for _, link := range entry.Links {
		attachment := hyperfeed.Attachment{}
		if link.Rel != "enclosure" {
			continue
		}
		attachment.URL, _ = href.Parse(link.Href)
		attachment.Title = link.Title
		attachment.Size = int64(link.Length)
		attachment.Mimetype = link.Type
		attachment.Duration = 0 // Atom links do not have a duration field
		attachment.Mimetype = link.Type
		attachment.ItemID = item.Identifier()
		attachment.FeedID = item.Feed.Identifier()
		attachment.WebsiteID = item.Feed.Website.Identifier()
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// JSONFeedParser.Ready()
func (parser *JSONFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// JSONFeedParser.Parse()
func (parser *JSONFeedParser) Parse(timelinePosition time.Time) (*hyperfeed.Feed, error) {
	var err error
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.JSON, err = jsonfeed.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}

	// Feed properties
	if parser.Feed.HTMLURL == nil || parser.Feed.URL.String() == parser.Feed.HTMLURL.String() {
		parser.Feed.HTMLURL, _ = href.Parse(parser.JSON.HomePageURL)
		parser.Feed.HTMLURL = parser.Feed.URL.ResolveReference(parser.Feed.HTMLURL.URL()) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.JSON.Title
	parser.Feed.Description = parser.JSON.Description
	parser.Feed.Language = parser.JSON.Language
	parser.Feed.Metadata = hyperfeed.Metadata{}

	// Feed.Website properties
	if parser.Feed.Website.FaviconURL == "" {
		parser.Feed.Website.FaviconURL = href.ResolveHrefString(parser.Feed.URL, hyperfeed.First(parser.JSON.Favicon, parser.JSON.Icon))
	}
	if parser.Feed.Website.CoverURL == "" {
		parser.Feed.Website.CoverURL = href.ResolveHrefString(parser.Feed.URL, parser.JSON.Icon)
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
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt, timelinePosition); err != nil {
		return nil, err
	}
	if len(parser.JSON.Items) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.JSON.Items), len(parser.Feed.Items)))
	}
	return parser.Feed, nil
}

// JSONFeedParser.Items()
func (parser *JSONFeedParser) Items(added time.Time, timelinePosition time.Time) (items []*hyperfeed.Item, err error) {
	for _, jsonitem := range parser.JSON.Items {
		item := &hyperfeed.Item{Feed: parser.Feed}
		item.URL, _ = url.Parse(hyperfeed.First(jsonitem.URL, jsonitem.ID)) // prefer URL, fallback to ID
		if !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		if jsonitem.ExternalURL != "" && jsonitem.ExternalURL != jsonitem.URL {
			item.ExternalHrefs = []string{jsonitem.ExternalURL}
		}
		publishedTime, _ := time.Parse(time.RFC3339, jsonitem.DatePublished)
		modifiedTime, _ := time.Parse(time.RFC3339, jsonitem.DateModified)
		item.Title = jsonitem.Title
		item.Images = hyperfeed.ParseImages(item, []byte(jsonfeed.HTML(hyperfeed.First(jsonitem.ContentHTML, jsonitem.ContentText, jsonitem.Summary))))
		if item.Summary, err = markdown.Convert(hyperfeed.First(jsonitem.Summary, jsonitem.ContentHTML, jsonitem.ContentText)); err != nil {
			fmt.Printf("markdown JSONFeed item.Summary error: %v\n", err)
		}
		if item.Content, err = markdown.Convert(jsonfeed.HTML(hyperfeed.First(jsonitem.ContentHTML, jsonitem.ContentText, jsonitem.Summary))); err != nil {
			fmt.Printf("markdown JSONFeed item.Content error: %v\n", err)
		}
		item.Comments = nil // Atom entries do not have a comments field
		item.PublishedAt = hyperfeed.Earliest(publishedTime, modifiedTime)
		item.ModifiedAt = hyperfeed.Latest(modifiedTime, publishedTime)
		item.AddedAt = added
		item.UpdatedAt = added
		item.FeedAt = item.TimelineAt(timelinePosition)
		for _, byline := range jsonitem.Authors {
			url, _ := href.Parse(byline.URL)
			favicon, _ := href.Parse(byline.Avatar)
			item.Bylines = append(item.Bylines, hyperfeed.Byline{Href: url, Name: byline.Name, Favicon: favicon})
		}
		if item.Attachments, err = parser.Attachments(&jsonitem, item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// JSONFeedParser.Attachments()
func (parser *JSONFeedParser) Attachments(jsonfeeditem *jsonfeed.Item, item *hyperfeed.Item) (attachments []hyperfeed.Attachment, err error) {
	for _, a := range jsonfeeditem.Attachments {
		attachment := hyperfeed.Attachment{}
		attachment.URL, _ = href.Parse(a.URL)
		attachment.Title = a.Title
		attachment.Size = int64(a.SizeInBytes)
		attachment.Mimetype = a.MimeType
		attachment.Duration = a.DurationInSeconds
		attachment.ItemID = item.Identifier()
		attachment.FeedID = item.Feed.Identifier()
		attachment.WebsiteID = item.Feed.Website.Identifier()
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// NoFeedParser.Ready()
func (parser *NoFeedParser) Ready() bool {
	return true
}

// NoFeedParser.Parse()
func (parser *NoFeedParser) Parse(_ time.Time) (*hyperfeed.Feed, error) {
	return parser.Feed, nil
}

// FUNCTIONS ===================================================================

// NewFeedParser detects and returns an appropriate FeedParserKind for the given feed bytes.
func NewFeedParser(feed *hyperfeed.Feed, bytes []byte) (parser FeedParser, err error) {
	if bytes == nil || feed == nil || feed.URL == nil {
		return nil, errors.Join(ErrParserInitialization, errors.New("nil feed passed to NewFeedParser()"))
	}
	switch {
	case len(bytes) == 0:
		feed.Kind = "unknown"
		parser = &NoFeedParser{Feed: feed}
	case rss.Detected(bytes):
		feed.Kind = "rss"
		parser = &RSSFeedParser{Bytes: bytes, Feed: feed}
	case atom.Detected(bytes):
		feed.Kind = "atom"
		parser = &AtomFeedParser{Bytes: bytes, Feed: feed}
	case jsonfeed.Detected(bytes):
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
