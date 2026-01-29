package rss

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// RSS represents an RSS feed (0.91/0.92/2.0).
// See https://www.rssboard.org/rss-specification
type RSS struct {
	Version string  `xml:"version,attr" json:"version,omitempty"`
	Channel Channel `xml:"channel"`

	Origin *url.URL `xml:"-" json:"-"`
}

type Channel struct {
	Title          string     `xml:"title"`
	Links          []Link     `xml:"link"`
	Description    string     `xml:"description"`
	Language       string     `xml:"language"`
	PubDate        string     `xml:"pubDate"`
	LastBuildDate  string     `xml:"lastBuildDate"`
	Copyright      string     `xml:"copyright"`
	ManagingEditor string     `xml:"managingEditor"`
	Webmaster      string     `xml:"webMaster"`
	Generator      string     `xml:"generator"`
	Docs           string     `xml:"docs"`
	TTL            int        `xml:"ttl"`
	Categories     []Category `xml:"category"`
	Cloud          *Cloud     `xml:"cloud"`
	Image          *Image     `xml:"image"`
	TextInput      *TextInput `xml:"textInput"`
	SkipHours      *Hours     `xml:"skipHours"`
	SkipDays       *Days      `xml:"skipDays"`
	Items          []Item     `xml:"item"`

	Extensions []Extension `xml:",any" json:"-"`
}

type Item struct {
	Links          []Link      `xml:"link"` // some feeds use multiple <link> elements (see NYT <atom:link> namespace elements)
	Title          string      `xml:"title"`
	Description    string      `xml:"description"`
	ContentEncoded string      `xml:"http://purl.org/rss/1.0/modules/content/ encoded,omitempty"` // RSS Content Module
	Authors        []string    `xml:"author"`
	Comments       Text        `xml:"comments"`
	GUID           RSSGUID     `xml:"guid"`
	PubDate        string      `xml:"pubDate"`
	Categories     []Category  `xml:"category"`
	Enclosures     []Enclosure `xml:"enclosure"`
	Source         *Source     `xml:"source"`

	// Extensions captures additional namespaced elements under <item>.
	Extensions []Extension `xml:",any" json:"-"`
}

type Link struct {
	Href  string `xml:",chardata"`
	Rel   string `xml:"rel,attr,omitempty"` // for Atom links
	Type  string `xml:"type,attr,omitempty"`
	Space string `xml:"xmlns,attr,omitempty"`

	uri *url.URL `xml:"-" json:"-"`
}

// Text represents character data elements with no namespace.
type Text struct {
	Value string `xml:",chardata"`
}

type AtomLink struct {
	Href     string `xml:"href,attr"`
	Rel      string `xml:"rel,attr,omitempty"`
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   uint   `xml:"length,attr,omitempty"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`    // URL of the enclosure
	Length string `xml:"length,attr"` // Size in bytes
	Type   string `xml:"type,attr"`   // MIME type
}

// RSSGUID may include the isPermaLink attribute.
type RSSGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr" json:"isPermaLink,omitempty"`
	Value       string `xml:",chardata" json:"value,omitempty"`
}

// Category allows the optional domain attribute.
type Category struct {
	Domain string `xml:"domain,attr" json:"domain,omitempty"`
	Text   string `xml:",chardata" json:"text,omitempty"`
}

// Source is an item source reference with a required url attribute.
type Source struct {
	URL  string `xml:"url,attr" json:"url,omitempty"`
	Text string `xml:",chardata" json:"text,omitempty"`
}

// Cloud configures real-time notifications per the spec.
type Cloud struct {
	Domain            string `xml:"domain,attr"`
	Port              int    `xml:"port,attr"`
	Path              string `xml:"path,attr"`
	RegisterProcedure string `xml:"registerProcedure,attr"`
	Protocol          string `xml:"protocol,attr"`
}

// Image describes the channel image.
type Image struct {
	URL         string `xml:"url"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Width       int    `xml:"width"`
	Height      int    `xml:"height"`
	Description string `xml:"description"`
}

// TextInput describes a text input form.
type TextInput struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Name        string `xml:"name"`
	Link        string `xml:"link"`
}

// Hours wraps skipHours/hour elements.
type Hours struct {
	Hours []int `xml:"hour"`
}

// RSSDays wraps skipDays/day elements.
type Days struct {
	Days []string `xml:"day"`
}

// Variables
// =============================================================================
var (
	ErrDateParser = fmt.Errorf("date parser error")

	emailAddressPattern *regexp.Regexp = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
)

// Methods
// =============================================================================

func (rss *RSS) FeedVersion() string {
	return rss.Version
}

func (rss *RSS) ItemLink(item *Item, base *url.URL) *url.URL {
	if item == nil || item.Links == nil {
		return nil
	}
	for _, link := range item.Links {
		if link.IsExternal(base) {
			continue
		}
		if link.URL() != nil {
			return base.ResolveReference(link.URL())
		}
	}
	if u, err := url.Parse(item.GUID.Value); err == nil && u != nil && item.GUID.IsPermaLink {
		return u
	}
	if u, err := url.Parse(item.Comments.Value); err == nil && u != nil && !(&Link{Href: item.Comments.Value}).IsExternal(base) {
		if u.Fragment == "comments" || u.Fragment == "respond" {
			u.Fragment = ""
		}
		return base.ResolveReference(u)
	}
	return nil
}

// Item.ExternalURLs() returns a slice of external URLs found in the item's links.
func (item *Item) ExternalURLs(base *url.URL) (urls []*url.URL) {
	if item == nil || item.Links == nil {
		return urls
	}
	for _, link := range item.Links {
		if u := link.URL(); u != nil && link.IsExternal(base) {
			urls = append(urls, u)
		}
	}
	return urls
}

// Link.URL()
func (link *Link) URL() (u *url.URL) {
	if link == nil || link.Href == "" {
		return nil
	}
	if link.uri == nil {
		link.uri, _ = url.Parse(link.Href) // ignore error, return nil on failure
	}
	return link.uri
}

// Link.IsExternal()
func (link *Link) IsExternal(base *url.URL) bool {
	if link == nil || link.URL() == nil || link.URL().Host == "" {
		return false
	}
	if base == nil {
		return true
	}
	if strings.EqualFold(link.Rel, "shorturl") {
		return false // DaringFireball "short URL" links
	}
	return !strings.EqualFold(strings.TrimPrefix(link.URL().Host, "www."), strings.TrimPrefix(base.Host, "www."))
}

func (link *Link) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	var atomLink AtomLink = AtomLink{}
	if link == nil {
		link = &Link{}
	}
	switch {
	case start.Name.Local == "link" && start.Name.Space == "http://www.w3.org/2005/Atom":
		// handle <atom:link> elements
		if err = d.DecodeElement(&atomLink, &start); err != nil {
			return err
		}
		link.Href = atomLink.Href
		link.Rel = atomLink.Rel
		link.Type = atomLink.Type
		link.Space = start.Name.Space
	case start.Name.Local == "link":
		// handle standard <link> elements
		if err = d.DecodeElement(&link.Href, &start); err != nil {
			return err
		}
	default:
		fmt.Printf("WARNING: unhandled rss.Link element: \"%s/%s\"\n", start.Name.Space, start.Name.Local)
		d.Skip()
	}
	return nil
}

func (text *Text) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	if start.Name.Space == "" || strings.EqualFold(start.Name.Space, ExtensionNamespaceRSS) {
		if err = d.DecodeElement(&text.Value, &start); err != nil {
			return err
		}
	} else {
		d.Skip()
	}
	return nil
}

// Functions
// =============================================================================

// rss.Detected()
func Detected(doc []byte) bool {
	b := bytes.TrimSpace(doc)
	return bytes.Contains(b, []byte("<rss"))
}

// rss.Parse()
func Parse(incoming []byte) (r *RSS, err error) {
	var decoder *xml.Decoder = xml.NewDecoder(bytes.NewReader(incoming))
	decoder.CharsetReader = charset.NewReaderLabel
	if err = decoder.Decode(&r); err != nil {
		return nil, err
	}
	return r, nil
}

// rss.Date() parses various RSS date formats into time.Time
func Date(from string) (t time.Time, err error) {
	if from == "" {
		return time.Time{}, nil
	}
	if t, err = time.Parse(time.RFC1123, from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse(time.RFC1123Z, from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse(time.RFC822, from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse(time.RFC822Z, from); err == nil {
		return t.UTC(), nil
	}
	// catch dates without zero-padded day
	if t, err = time.Parse("Mon, 2 Jan 2006 15:04:05 MST", from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", from); err == nil {
		return t.UTC(), nil
	}
	// catch non-standard ISO 8601 dates, tisk tisk
	if t, err = time.Parse(time.RFC3339, from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05-0700", from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05Z0700", from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05.000Z0700", from); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05.000-0700", from); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, ErrDateParser
}

func (r *RSS) HTMLHref() string {
	if r == nil {
		return ""
	}
	for _, link := range r.Channel.Links {
		if strings.EqualFold(link.Rel, "self") || link.Type == "application/rss+xml" {
			continue // skip self-referencing feed links
		}
		if strings.EqualFold(link.Rel, "hub") {
			continue // skip WebSub links (https://www.w3.org/TR/websub/)
		}
		if link.Href == r.Origin.String() {
			continue // skip links that match the feed URL
		}
		if link.Href == "" {
			continue // skip empty links
		}
		return link.Href
	}
	for i, item := range r.Channel.Items {
		if i == 3 {
			break
		}
		if itemLink := r.ItemLink(&item, r.Origin); itemLink != nil {
			itemLink.Path = "/"
			return itemLink.String()
		}
	}
	return r.Origin.ResolveReference(&url.URL{Path: "/"}).String()
}

func (r *RSS) IsPodcast() bool {
	if len(GetExtensionProperties(r.Channel.Extensions, ExtensionNamespaceItunesPodcast)) > 0 {
		return true
	}
	for _, item := range r.Channel.Items {
		if item.IsPodcastEpisode() {
			return true
		}
	}
	return false
}

func (item *Item) IsPodcastEpisode() bool {
	if item == nil {
		return false
	}
	for _, enclosure := range item.Enclosures {
		if strings.HasPrefix(enclosure.Type, "audio/") || strings.HasPrefix(enclosure.Type, "video/") {
			return true
		}
	}
	return false
}

func (item *Item) PodcastDescription() string {
	if item == nil {
		return ""
	}
	if desc := GetExtensionProperty(item.Extensions, ExtensionNamespaceItunesPodcast, "summary"); desc != nil {
		return desc.Text
	}
	if desc := GetExtensionProperty(item.Extensions, ExtensionNamespaceItunesPodcast, "subtitle"); desc != nil {
		return desc.Text
	}
	return item.Description
}

// ParseAuthor extracts name and email from an RSS author string.
//
// NOTE: <author> elements are not well-defined in RSS 2.0 and may contain just an email, just a name, or both in various formats.
//
// The following formats are supported:
// * "email"
// * "email <name>"
// * "email (name)"
// * "email [name]"
// * "email {name}"
// * "name"
// * "name <email>"
// * "name (email)"
// * "name [email]"
// * "name {email}"
func ParseAuthor(incoming string) (name, email string) {
	var parts []string = strings.Fields(strings.TrimSpace(incoming))
	for i, part := range parts {
		if emailAddressPattern.MatchString(part) {
			email = strings.TrimSpace(part)
			email = strings.Trim(email, "<>(){}[]")
			parts = slices.Delete(parts, i, i+1)
			break
		}
	}
	name = strings.TrimSpace(strings.Join(parts, " "))
	name = strings.Trim(name, "<>(){}[]")
	return name, email
}

func HTML(text string) string {
	// if text != "" && !htmlTagPattern.MatchString(text) {
	// 	return fmt.Sprintf("<p>%s</p>", text)
	// }
	return text
}
