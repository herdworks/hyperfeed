package feeder

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	"github.com/herdworks/hyperfeed-go/types/atom"
	"github.com/herdworks/hyperfeed-go/types/href"
	"github.com/herdworks/hyperfeed-go/types/jsonfeed"
	"github.com/herdworks/hyperfeed-go/types/rss"
	hypergraph "github.com/herdworks/hypergraph-go"
)

// TYPES
// =============================================================================

type FeedParser interface {
	Ready() bool
	Parse() (*hyperfeed.Feed, error)
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

// RawFeed represents a format-specific parsed feed (RSS/Atom/JSON Feed) before normalization.
// Implementations should report their concrete kind and version via Kind().
type RawFeed interface {
	// Kind() FeedKind                                          // returns the concrete feed kind/version (e.g., rss/2.0, jsonfeed/1.1)
	GetURL() *url.URL                                                     // returns the feed's base URL (for resolving relative item URLs)
	SetURL(*url.URL)                                                      // set the raw feed's base URL (for resolving relative item URLs)
	ItemCount() int                                                       // returns the number of items in the feed
	ParseFeedAuthors() []hyperfeed.Author                                 // parses and returns the feed's owners
	ParseItems(feedID string, added time.Time) ([]*hyperfeed.Item, error) // parses and returns the feed's items
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
func (parser *RSSFeedParser) Parse() (*hyperfeed.Feed, error) {
	var err error
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.RSS, err = rss.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}
	if parser.Feed.HTMLURL == nil || parser.Feed.URL.String() == parser.Feed.HTMLURL.String() {
		parser.Feed.HTMLURL, _ = href.Parse(parser.RSS.Link())
	}
	if parser.Feed.HTMLURL != nil && !parser.Feed.HTMLURL.IsAbs() {
		parser.Feed.HTMLURL = parser.Feed.URL.ResolveReference(parser.Feed.HTMLURL.URL()) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.RSS.Channel.Title
	parser.Feed.Description = parser.RSS.Channel.Description
	parser.Feed.Language = parser.RSS.Channel.Language
	authors := []string{parser.RSS.Channel.ManagingEditor, parser.RSS.Channel.Webmaster, rss.GetItunesOwner(parser.RSS.Channel.Extensions)}
	parser.Feed.Authors = append(parser.Feed.Authors, parser.Authors(authors)...) // append in case parser was initialized with an author
	parser.Feed.Metadata = hyperfeed.Metadata{}
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt); err != nil {
		return nil, err
	}
	if len(parser.RSS.Channel.Items) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.RSS.Channel.Items), len(parser.Feed.Items)))
	}
	return parser.Feed, nil
}

// RSSFeedParser.Authors()
func (parser *RSSFeedParser) Authors(incoming []string) (authors []hyperfeed.Author) {
	for _, a := range incoming {
		if len(a) == 0 {
			continue
		}
		var author hyperfeed.Author = hyperfeed.Author{
			// Author default values
			Href:     parser.Feed.HTMLURL,
			Name:     parser.Feed.Title,
			Username: hypergraph.Username(parser.Feed.HTMLURL.String()),
			Favicon:  parser.Feed.FaviconURL,
			Source:   "rss",
		}
		name, email := rss.ParseAuthor(a)
		if name != "" {
			author.Name = name
		}
		if email != "" {
			author.Email = email
		}
		if a == parser.RSS.Channel.ManagingEditor {
			author.Source += "/managing_editor"
		} else if a == parser.RSS.Channel.Webmaster {
			author.Source += "/webmaster"
		}
		authors = append(authors, author)
	}
	return authors
}

// RSSFeedParser.Items()
func (parser *RSSFeedParser) Items(added time.Time) (items []*hyperfeed.Item, err error) {
	for _, rssitem := range parser.RSS.Channel.Items {
		item := &hyperfeed.Item{Feed: parser.Feed}
		if item.URL = rss.LinkURL(rssitem.Links); item.URL != nil && !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		if !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		item.FeedID = item.Feed.Identifier()
		item.Title = rssitem.Title
		item.Summary = rssitem.Description
		item.Content = rssitem.ContentEncoded
		item.Comments, _ = url.Parse(rssitem.Comments)
		item.PublishedAt, _ = rss.Date(rssitem.PubDate)
		item.ModifiedAt = item.PublishedAt // rss items don't have a modified date
		item.AddedAt = added
		item.UpdatedAt = added
		if byline := rss.GetDublinCoreCreator(rssitem.Extensions); byline != "" {
			name, email := rss.ParseAuthor(byline)
			item.Bylines = append(item.Bylines, hyperfeed.Author{Name: name, Email: email, Source: "rss/byline"})
		}
		if item.Attachments, err = parser.Attachments(&rssitem, item.Identifier()); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// RSSFeedParser.Attachments()
func (parser *RSSFeedParser) Attachments(item *rss.Item, itemID string) (attachments []hyperfeed.Attachment, err error) {
	for _, enclosure := range item.Enclosures {
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
		if duration := rss.GetItunesMetadata(item.Extensions, "duration"); duration != nil {
			if v, err := time.Parse("15:04:05", duration.Text); err == nil {
				attachment.Duration = int64(v.Hour()*3600 + v.Minute()*60 + v.Second()) // duration in seconds
			}
		}
		attachment.Mimetype = enclosure.Type
		attachment.ItemID = itemID
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// AtomFeedParser.Ready()
func (parser *AtomFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// AtomFeedParser.Parse()
func (parser *AtomFeedParser) Parse() (*hyperfeed.Feed, error) {
	var err error
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.Atom, err = atom.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}
	if parser.Feed.HTMLURL == nil || parser.Feed.URL.String() == parser.Feed.HTMLURL.String() {
		if link := parser.Atom.Link("alternate"); link != nil {
			parser.Feed.HTMLURL, _ = href.Parse(link.Href)
		}
	}
	if parser.Feed.HTMLURL != nil && !parser.Feed.HTMLURL.IsAbs() {
		parser.Feed.HTMLURL = parser.Feed.URL.ResolveReference(parser.Feed.HTMLURL.URL()) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.Atom.Title
	parser.Feed.Description = ""
	parser.Feed.Language = ""
	parser.Feed.Authors = append(parser.Feed.Authors, parser.Authors(parser.Atom.Authors...)...) // append in case parser was initialized with an author
	parser.Feed.Metadata = hyperfeed.Metadata{}
	parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt)
	if len(parser.Atom.Entries) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.Atom.Entries), len(parser.Feed.Items)))
	}
	return parser.Feed, err
}

// AtomFeedParser.Authors()
func (parser *AtomFeedParser) Authors(incoming ...*atom.Author) (authors []hyperfeed.Author) {
	for _, a := range incoming {
		var author hyperfeed.Author = hyperfeed.Author{
			// Author default values
			Href:     parser.Feed.HTMLURL,
			Name:     parser.Feed.Title,
			Username: hypergraph.Username(parser.Feed.HTMLURL.String()),
			Favicon:  parser.Feed.FaviconURL,
			Source:   "atom",
		}
		if a.URI != "" {
			author.Href, _ = href.Parse(a.URI)
			author.Username = hypergraph.Username(a.URI)
		}
		if a.Name != "" {
			author.Name = a.Name
		}
		if a.Email != "" {
			author.Email = a.Email
		}
		authors = append(authors, author)
	}
	return authors
}

// AtomFeedParser.Items()
func (parser *AtomFeedParser) Items(added time.Time) (items []*hyperfeed.Item, err error) {
	for _, entry := range parser.Atom.Entries {
		item := &hyperfeed.Item{Feed: parser.Feed}
		if link := entry.Link("alternate"); link != nil {
			item.URL, _ = url.Parse(link.Href)
		}
		if !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		item.FeedID = item.Feed.Identifier()
		item.Title = entry.Title
		item.Summary = atom.FirstTextBody(entry.Summary, entry.Content) // prefer summary, fallback to content
		item.Content = atom.FirstTextBody(entry.Content, entry.Summary) // prefer content, fallback to summary
		item.Comments = nil                                             // Atom entries do not have a comments field
		item.PublishedAt, _ = time.Parse(time.RFC3339, entry.Published)
		item.ModifiedAt, _ = time.Parse(time.RFC3339, entry.Updated)
		item.AddedAt = added
		item.UpdatedAt = added
		for _, byline := range append(entry.Authors, entry.Contributors...) {
			href, _ := href.Parse(byline.URI)
			item.Bylines = append(item.Bylines, hyperfeed.Author{Href: href, Name: byline.Name, Email: byline.Email, Source: "atom/byline"})
		}
		if item.Attachments, err = parser.Attachments(entry, item.Identifier()); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// AtomFeedParser.Attachments()
func (parser *AtomFeedParser) Attachments(item *atom.Entry, itemID string) (attachments []hyperfeed.Attachment, err error) {
	for _, link := range item.Links {
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
		attachment.ItemID = itemID
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// JSONFeedParser.Ready()
func (parser *JSONFeedParser) Ready() bool {
	return parser.Bytes != nil && parser.Feed != nil && parser.Feed.URL != nil
}

// JSONFeedParser.Parse()
func (parser *JSONFeedParser) Parse() (*hyperfeed.Feed, error) {
	var err error
	if !parser.Ready() {
		return nil, ErrParserInitialization
	}
	if parser.JSON, err = jsonfeed.Parse(parser.Bytes); err != nil {
		return nil, errors.Join(ErrParsingRawFeed, err)
	}
	if parser.Feed.HTMLURL == nil || parser.Feed.URL.String() == parser.Feed.HTMLURL.String() {
		parser.Feed.HTMLURL, _ = href.Parse(parser.JSON.HomePageURL)
	}
	if parser.Feed.HTMLURL != nil && !parser.Feed.HTMLURL.IsAbs() {
		parser.Feed.HTMLURL = parser.Feed.URL.ResolveReference(parser.Feed.HTMLURL.URL()) // resolve relative URLs against the feed URL
	}
	parser.Feed.Title = parser.JSON.Title
	parser.Feed.Description = parser.JSON.Description
	parser.Feed.Language = parser.JSON.Language
	parser.Feed.Authors = append(parser.Feed.Authors, parser.Authors(parser.JSON.Authors)...) // append in case parser was initialized with an author
	parser.Feed.Metadata = hyperfeed.Metadata{}
	if parser.Feed.Items, err = parser.Items(parser.Feed.Status.UpdatedAt); err != nil {
		return nil, err
	}
	if len(parser.JSON.Items) != len(parser.Feed.Items) {
		return nil, errors.Join(ErrParsingRawFeed, fmt.Errorf("item count mismatch: expected %d, got %d", len(parser.JSON.Items), len(parser.Feed.Items)))
	}
	return parser.Feed, nil
}

// JSONFeedParser.Authors()
func (parser *JSONFeedParser) Authors(incoming []jsonfeed.Author) (authors []hyperfeed.Author) {
	for _, a := range incoming {
		var author hyperfeed.Author = hyperfeed.Author{
			// Author default values
			Href:     parser.Feed.HTMLURL,
			Name:     parser.Feed.Title,
			Username: hypergraph.Username(parser.Feed.HTMLURL.String()),
			Favicon:  parser.Feed.FaviconURL,
			Source:   "jsonfeed",
		}
		if a.URL != "" {
			author.Href, _ = href.Parse(a.URL)
			author.Username = hypergraph.Username(a.URL)
		}
		if a.Name != "" {
			author.Name = a.Name
		}
		if a.Avatar != "" {
			author.Favicon, _ = href.Parse(a.Avatar)
		}
		authors = append(authors, author)
	}
	return authors
}

// JSONFeedParser.Items()
func (parser *JSONFeedParser) Items(added time.Time) (items []*hyperfeed.Item, err error) {
	for _, jsonitem := range parser.JSON.Items {
		item := &hyperfeed.Item{Feed: parser.Feed}
		item.URL, _ = url.Parse(hyperfeed.First(jsonitem.URL, jsonitem.ID)) // prefer URL, fallback to ID
		if !item.URL.IsAbs() {
			item.URL = parser.Feed.URL.ResolveReference(item.URL).URL() // resolve relative URLs against the feed URL
		}
		item.FeedID = item.Feed.Identifier()
		item.Title = jsonitem.Title
		item.Summary = hyperfeed.First(jsonitem.Summary, jsonitem.ContentHTML, jsonitem.ContentText) // prefer summary, fallback to content
		item.Content = hyperfeed.First(jsonitem.ContentHTML, jsonitem.ContentText, jsonitem.Summary) // prefer content, fallback to summary
		item.Comments = nil                                                                          // Atom entries do not have a comments field
		item.PublishedAt, _ = time.Parse(time.RFC3339, jsonitem.DatePublished)
		item.ModifiedAt, _ = time.Parse(time.RFC3339, jsonitem.DateModified)
		item.AddedAt = added
		item.UpdatedAt = added
		for _, byline := range jsonitem.Authors {
			url, _ := href.Parse(byline.URL)
			favicon, _ := href.Parse(byline.Avatar)
			item.Bylines = append(item.Bylines, hyperfeed.Author{Href: url, Name: byline.Name, Favicon: favicon, Source: "jsonfeed/byline"})
		}
		if item.Attachments, err = parser.Attachments(&jsonitem, item.Identifier()); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// JSONFeedParser.Attachments()
func (parser *JSONFeedParser) Attachments(item *jsonfeed.Item, identifier string) (attachments []hyperfeed.Attachment, err error) {
	for _, a := range item.Attachments {
		attachment := hyperfeed.Attachment{}
		attachment.ItemID = identifier
		attachment.URL, _ = href.Parse(a.URL)
		attachment.Title = a.Title
		attachment.Size = int64(a.SizeInBytes)
		attachment.Mimetype = a.MimeType
		attachment.Duration = a.DurationInSeconds
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// NoFeedParser.Ready()
func (parser *NoFeedParser) Ready() bool {
	return true
}

// NoFeedParser.Parse()
func (parser *NoFeedParser) Parse() (*hyperfeed.Feed, error) {
	return parser.Feed, nil
}

// FUNCTIONS ===================================================================

// NewFeedParser detects and returns an appropriate FeedParserKind for the given feed bytes.
func NewFeedParser(origin *url.URL, bytes []byte, feed *hyperfeed.Feed) (parser FeedParser) {
	u := href.Href(*origin)
	if feed == nil {
		feed = &hyperfeed.Feed{URL: &u}
	}
	if feed.URL == nil {
		feed.URL = &u
	}
	if feed.Status == nil {
		feed.Status = &hyperfeed.FeedStatus{}
	}
	switch {
	case len(bytes) == 0:
		parser = &NoFeedParser{Feed: feed}
	case rss.Detected(bytes):
		parser = &RSSFeedParser{Bytes: bytes, Feed: feed}
	case atom.Detected(bytes):
		parser = &AtomFeedParser{Bytes: bytes, Feed: feed}
	case jsonfeed.Detected(bytes):
		parser = &JSONFeedParser{Bytes: bytes, Feed: feed}
	}
	return parser
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
