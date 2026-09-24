package rss

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed/hosted"
	"github.com/herdworks/hyperfeed/xmlns"
	"golang.org/x/net/html/charset"
)

// TYPES
// =============================================================================

// RSS represents an RSS feed (0.91/0.92/2.0).
// See https://www.rssboard.org/rss-specification
type RSS struct {
	XMLName xml.Name `json:"-"`
	Version string   `xml:"version,attr" json:"version,omitempty"`

	AtomNS    string `xml:"xmlns:atom,attr,omitempty" json:"-"`
	ContentNS string `xml:"xmlns:content,attr,omitempty" json:"-"`
	DublinNS  string `xml:"xmlns:dc,attr,omitempty" json:"-"`
	ItunesNS  string `xml:"xmlns:itunes,attr,omitempty" json:"-"`
	MediaNS   string `xml:"xmlns:media,attr,omitempty" json:"-"`
	PodcastNS string `xml:"xmlns:podcast,attr,omitempty" json:"-"`

	Channel Channel `xml:"channel"`

	Origin *url.URL `xml:"-" json:"-"`
}

type Channel struct {
	Title          string     `xml:"title"`
	Links          []Link     `xml:"link"`
	Description    string     `xml:"description"`
	Language       string     `xml:"language,omitempty"`
	PubDate        string     `xml:"pubDate,omitempty"`
	LastBuildDate  string     `xml:"lastBuildDate,omitempty"`
	Copyright      string     `xml:"copyright,omitempty"`
	ManagingEditor string     `xml:"managingEditor,omitempty"`
	Webmaster      string     `xml:"webMaster,omitempty"`
	Generator      string     `xml:"generator,omitempty"`
	Docs           string     `xml:"docs,omitempty"`
	TTL            int        `xml:"ttl,omitempty"`
	Categories     []Category `xml:"category,omitempty"`
	Cloud          *Cloud     `xml:"cloud"`
	Image          *Image     `xml:"image"`
	TextInput      *TextInput `xml:"textInput"`
	SkipHours      *Hours     `xml:"skipHours"`
	SkipDays       *Days      `xml:"skipDays"`
	Items          []Item     `xml:"item,omitempty"`

	Extensions xmlns.Extensions `xml:",any" json:"-"`
}

type Item struct {
	Links          []Link      `xml:"link,omitempty"` // some feeds use multiple <link> elements (see NYT <atom:link> namespace elements)
	Title          string      `xml:"title"`
	Description    string      `xml:"description,omitempty"`
	ContentEncoded string      `xml:"http://purl.org/rss/1.0/modules/content/ encoded,omitempty"` // RSS Content Module
	Authors        []string    `xml:"author,omitempty"`
	Comments       Text        `xml:"comments"`
	GUID           RSSGUID     `xml:"guid"`
	PubDate        string      `xml:"pubDate,omitempty"`
	Categories     []Category  `xml:"category,omitempty"`
	Enclosures     []Enclosure `xml:"enclosure,omitempty"`
	Source         *Source     `xml:"source"`

	// Extensions captures additional namespaced elements under <item>.
	Extensions xmlns.Extensions `xml:",any" json:"-"`
}

type Link struct {
	Href     string `xml:",chardata"`
	Rel      string `xml:"rel,attr,omitempty"` // for Atom links
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Space    string `xml:"xmlns,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   *uint  `xml:"length,attr,omitempty"`

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
	Length   *uint  `xml:"length,attr,omitempty"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`    // URL of the enclosure
	Length string `xml:"length,attr"` // Size in bytes
	Type   string `xml:"type,attr"`   // MIME type
}

// RSSGUID may include the isPermaLink attribute.
type RSSGUID struct {
	IsPermaLink bool   `xml:"-" json:"isPermaLink,omitempty"`
	Value       string `xml:",chardata" json:"value,omitempty"`
}

// Category allows the optional domain attribute.
type Category struct {
	Domain string `xml:"domain,attr,omitempty" json:"domain,omitempty"`
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
	Width       int    `xml:"width,omitempty"`
	Height      int    `xml:"height,omitempty"`
	Description string `xml:"description,omitempty"`
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

// VARIABLES
// =============================================================================
var (
	ErrDateParser = fmt.Errorf("date parser error")

	emailAddressPattern *regexp.Regexp = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
)

// METHODS
// =============================================================================

func (rss *RSS) FeedVersion() string {
	return rss.Version
}

func (rss *RSS) ItemLink(item *Item, base *url.URL) *url.URL {
	if item == nil {
		return nil
	}
	for _, link := range item.Links {
		if link.IsEnclosure() {
			continue
		}
		if link.IsExternal(base) {
			continue
		}
		if link.URL() != nil {
			return base.ResolveReference(link.URL())
		}
	}
	if link := (Link{Href: item.GUID.Value, Rel: "guid"}); item.GUID.IsPermaLink && !link.IsExternal(base) {
		return link.URL()
	}
	if item.Comments.Value == "" {
		return nil
	}
	if u, err := url.Parse(item.Comments.Value); err == nil && u != nil && !(&Link{Href: item.Comments.Value}).IsExternal(base) {
		if u.Fragment == "comments" || u.Fragment == "respond" {
			u.Fragment = ""
		}
		return base.ResolveReference(u)
	}
	return nil
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

func (channel *Channel) GetEmail() (email string) {
	if _, email := ParseAuthor(channel.ManagingEditor); email != "" {
		return email
	} else if owner := channel.Extensions.GetItunesMetadata("owner"); owner != nil {
		if email := owner.Extensions.GetItunesMetadata("email"); email != nil {
			return email.Text
		}
	}
	return
}

// Item.ExternalURLs() returns a slice of external URLs found in the item's links.
func (item *Item) ExternalURLs(base *url.URL) (urls []*url.URL) {
	if item == nil {
		return urls
	}
	if item.Source != nil && item.Source.URL != "" {
		link := Link{Href: item.Source.URL}
		if u := link.URL(); u != nil && link.IsExternal(base) {
			urls = append(urls, u)
		}
	}
	for _, link := range item.Links {
		if link.IsEnclosure() || strings.EqualFold(link.Rel, "comments") || strings.EqualFold(link.Rel, "replies") {
			continue
		}
		if u := link.URL(); u != nil && link.IsExternal(base) {
			urls = append(urls, u)
		}
	}
	return urls
}

func (item *Item) PodcastDescription() string {
	if item == nil {
		return ""
	}
	if desc := item.Extensions.GetProperty(xmlns.ItunesPodcastNS, "summary"); desc != nil {
		return desc.Text
	}
	if desc := item.Extensions.GetProperty(xmlns.ItunesPodcastNS, "subtitle"); desc != nil {
		return desc.Text
	}
	return item.Description
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

func (link *Link) IsEnclosure() bool {
	return link != nil && strings.EqualFold(link.Rel, "enclosure")
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
	if strings.EqualFold(hosted.URL(link.URL()).Hostname(), strings.TrimPrefix(base.Hostname(), "www.")) {
		link.Href = hosted.URL(link.URL()).String() // normalize the link URL to improve matching with other links
		return false                                // medium.com and other hosted urls
	}
	return !strings.EqualFold(strings.TrimPrefix(link.URL().Host, "www."), strings.TrimPrefix(base.Host, "www."))
}

func (link Link) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if link.Space != xmlns.AtomNS {
		start.Name = xml.Name{Local: "link"}
		return e.EncodeElement(link.Href, start)
	}
	start.Name = xml.Name{Local: "atom:link"}
	start.Attr = append(start.Attr,
		xml.Attr{Name: xml.Name{Local: "href"}, Value: link.Href},
	)
	if link.Rel != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "rel"}, Value: link.Rel})
	}
	if link.Type != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "type"}, Value: link.Type})
	}
	if link.HrefLang != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "hreflang"}, Value: link.HrefLang})
	}
	if link.Title != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "title"}, Value: link.Title})
	}
	if link.Length != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "length"}, Value: strconv.FormatUint(uint64(*link.Length), 10)})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
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
		link.HrefLang = atomLink.HrefLang
		link.Title = atomLink.Title
		link.Length = atomLink.Length
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

func (guid RSSGUID) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if guid.Value == "" {
		return nil
	}
	start.Name = xml.Name{Local: "guid"}
	if !guid.IsPermaLink {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "isPermaLink"}, Value: "false"})
	}
	return e.EncodeElement(guid.Value, start)
}

func (guid *RSSGUID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	guid.IsPermaLink = true
	for _, attribute := range start.Attr {
		if !strings.EqualFold(attribute.Name.Local, "isPermaLink") {
			continue
		}
		if guid.IsPermaLink, err = strconv.ParseBool(attribute.Value); err != nil {
			return err
		}
		break
	}
	return d.DecodeElement(&guid.Value, &start)
}

func (text Text) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if text.Value == "" {
		return nil
	}
	return e.EncodeElement(text.Value, start)
}

func (text *Text) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	if start.Name.Space == "" || strings.EqualFold(start.Name.Space, xmlns.RSSNS) {
		if err = d.DecodeElement(&text.Value, &start); err != nil {
			return err
		}
	} else {
		d.Skip()
	}
	return nil
}

// FUNCTIONS
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

func GetItunesOwner(metadata xmlns.Extensions) (author string) {
	var name, email string
	if ownerMetadata := metadata.GetItunesMetadata("owner"); ownerMetadata != nil {
		if n := ownerMetadata.Extensions.GetItunesMetadata("name"); n != nil {
			name = n.Text
		}
		if e := ownerMetadata.Extensions.GetItunesMetadata("email"); e != nil {
			email = e.Text
		}
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func GetPodcastEpisodeDurationSeconds(item *Item) (seconds int) {
	var duration *xmlns.Extension
	if duration = item.Extensions.GetItunesMetadata("duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	} else if duration = item.Extensions.GetPodcastIndexMetadata("duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	} else if duration = item.Extensions.GetYahooMediaRSSMetadata("duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	} else if duration = item.Extensions.GetGooglePlayMetadata("duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	}
	return 0
}
func GetItunesDurationSeconds(duration *xmlns.Extension) (seconds int) {
	if duration == nil {
		return 0
	}
	switch d := GetItunesDurationDigits(duration.Text); len(d) {
	case 3:
		// HH:MM:SS format
		seconds += d[0] * 3600
		seconds += d[1] * 60
		seconds += d[2]
		return seconds
	case 2:
		// MM:SS format
		seconds += d[0] * 60
		seconds += d[1]
		return seconds
	case 1:
		// SS format
		return d[0]
	default:
		return 0
	}
}

func GetItunesDurationDigits(duration string) (parts []int) {
	for _, part := range strings.Split(duration, ":") {
		if p, err := strconv.Atoi(part); err != nil {
			return []int{0}
		} else {
			parts = append(parts, p)
		}
	}
	return parts
}

func GetPodcastCoverArtwork(channel *Channel, item *Item) (href string) {
	// check item-level artwork first
	if iTunesImage := item.Extensions.GetItunesMetadata("image"); iTunesImage != nil && iTunesImage.HasAttribute("href") {
		return iTunesImage.GetAttribute("href")
	}
	if podcastImage := item.Extensions.GetPodcastIndexMetadata("image"); podcastImage != nil && podcastImage.HasAttribute("href") {
		return podcastImage.GetAttribute("href")
	}
	if mediaThumbnail := item.Extensions.GetYahooMediaRSSMetadata("thumbnail"); mediaThumbnail != nil && mediaThumbnail.HasAttribute("url") {
		return mediaThumbnail.GetAttribute("url")
	}

	// use channel-level artwork as fallback
	if channel.Image != nil && channel.Image.URL != "" {
		return channel.Image.URL
	}
	if iTunesImage := channel.Extensions.GetProperty(xmlns.ItunesPodcastNS, "image"); iTunesImage != nil && iTunesImage.HasAttribute("href") {
		return iTunesImage.GetAttribute("href")
	}
	if podcastImage := channel.Extensions.GetProperty(xmlns.PodcastIndexNS, "image"); podcastImage != nil && podcastImage.HasAttribute("href") {
		return podcastImage.GetAttribute("href")
	}
	if mediaThumbnail := channel.Extensions.GetProperty(xmlns.YahooMediaRSSNS, "thumbnail"); mediaThumbnail != nil && mediaThumbnail.HasAttribute("url") {
		return mediaThumbnail.GetAttribute("url")
	}
	return ""
}
