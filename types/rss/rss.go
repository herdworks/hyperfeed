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
	Comments       string      `xml:"comments"`
	GUID           *RSSGUID    `xml:"guid"`
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
	IsPermaLink string `xml:"isPermaLink,attr" json:"isPermaLink,omitempty"`
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
)

// Methods
// =============================================================================

func (rss *RSS) FeedVersion() string {
	return rss.Version
}

func (rsslink *Link) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	var atomLink AtomLink = AtomLink{}
	if rsslink == nil {
		rsslink = &Link{}
	}
	switch {
	case start.Name.Local == "link" && start.Name.Space == "http://www.w3.org/2005/Atom":
		// handle <atom:link> elements
		if err = d.DecodeElement(&atomLink, &start); err != nil {
			return err
		}
		rsslink.Href = atomLink.Href
		rsslink.Rel = atomLink.Rel
		rsslink.Type = atomLink.Type
		rsslink.Space = start.Name.Space
	case start.Name.Local == "link":
		// handle standard <link> elements
		if err = d.DecodeElement(&rsslink.Href, &start); err != nil {
			return err
		}
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

func (r *RSS) Link() string {
	if r == nil {
		return ""
	}
	for _, link := range r.Channel.Links {
		if link.Href != "" {
			return link.Href
		}
	}
	return ""
}
func (r *RSS) GetURL() *url.URL {
	if r == nil {
		return nil
	}
	if u, err := url.Parse(r.Link()); err == nil {
		return u
	}
	return nil
}
func (r *RSS) SetURL(u *url.URL) {
	if u == nil || u.String() == "" {
		return
	}
	r.Channel.Links = append([]Link{{Href: u.String()}}, r.Channel.Links...) // prepend if not already present
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

func LinkURL(links []Link) *url.URL {
	for _, link := range links {
		if link.Href != "" {
			if u, err := url.Parse(link.Href); err == nil {
				return u
			}
		}
	}
	return nil
}
