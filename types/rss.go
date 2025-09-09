package hyperfeed

import (
	"bytes"
	"encoding/xml"

	"golang.org/x/net/html/charset"
)

// RSS represents an RSS feed (0.91/0.92/2.0).
// See https://www.rssboard.org/rss-specification
type RSS struct {
	Version string     `xml:"version,attr" json:"version,omitempty"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title          string        `xml:"title"`
	Link           string        `xml:"link"`
	Description    string        `xml:"description"`
	Language       string        `xml:"language"`
	PubDate        string        `xml:"pubDate"`
	LastBuildDate  string        `xml:"lastBuildDate"`
	Copyright      string        `xml:"copyright"`
	ManagingEditor string        `xml:"managingEditor"`
	Webmaster      string        `xml:"webMaster"`
	Generator      string        `xml:"generator"`
	Docs           string        `xml:"docs"`
	TTL            int           `xml:"ttl"`
	Categories     []RSSCategory `xml:"category"`
	Cloud          *RSSCloud     `xml:"cloud"`
	Image          *RSSImage     `xml:"image"`
	TextInput      *RSSTextInput `xml:"textInput"`
	SkipHours      *RSSHours     `xml:"skipHours"`
	SkipDays       *RSSDays      `xml:"skipDays"`
	Items          []RSSItem     `xml:"item"`

	Extensions []Extension `xml:",any" json:"-"`
}

type RSSItem struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	Author      string        `xml:"author"`
	Comments    string        `xml:"comments"`
	GUID        *RSSGUID      `xml:"guid"`
	PubDate     string        `xml:"pubDate"`
	Categories  []RSSCategory `xml:"category"`
	Enclosure   *RSSEnclosure `xml:"enclosure"`
	Source      *RSSSource    `xml:"source"`

	// Extensions captures additional namespaced elements under <item>.
	Extensions []Extension `xml:",any" json:"-"`
}

type RSSEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// RSSGUID may include the isPermaLink attribute.
type RSSGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr" json:"isPermaLink,omitempty"`
	Value       string `xml:",chardata" json:"value,omitempty"`
}

// RSSCategory allows the optional domain attribute.
type RSSCategory struct {
	Domain string `xml:"domain,attr" json:"domain,omitempty"`
	Text   string `xml:",chardata" json:"text,omitempty"`
}

// RSSSource is an item source reference with a required url attribute.
type RSSSource struct {
	URL  string `xml:"url,attr" json:"url,omitempty"`
	Text string `xml:",chardata" json:"text,omitempty"`
}

// RSSCloud configures real-time notifications per the spec.
type RSSCloud struct {
	Domain            string `xml:"domain,attr"`
	Port              int    `xml:"port,attr"`
	Path              string `xml:"path,attr"`
	RegisterProcedure string `xml:"registerProcedure,attr"`
	Protocol          string `xml:"protocol,attr"`
}

// RSSImage describes the channel image.
type RSSImage struct {
	URL         string `xml:"url"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Width       int    `xml:"width"`
	Height      int    `xml:"height"`
	Description string `xml:"description"`
}

// RSSTextInput describes a text input form.
type RSSTextInput struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Name        string `xml:"name"`
	Link        string `xml:"link"`
}

// RSSHours wraps skipHours/hour elements.
type RSSHours struct {
	Hours []int `xml:"hour"`
}

// RSSDays wraps skipDays/day elements.
type RSSDays struct {
	Days []string `xml:"day"`
}

// RawFeed.Kind() compliance
func (r *RSS) Kind() FeedKind {
	return FeedKindRSS_2_0
}

// RSSDetected returns true if the document appears to be RSS.
func RSSDetected(doc []byte) bool {
	b := bytes.TrimSpace(doc)
	return bytes.Contains(b, []byte("<rss"))
}

// ParseRSS parses raw RSS XML into an RSS struct.
func ParseRSS(doc []byte) (*RSS, error) {
	dec := xml.NewDecoder(bytes.NewReader(doc))
	dec.CharsetReader = charset.NewReaderLabel
	var r RSS
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Extension type and its UnmarshalXML are defined in types/metadata.go
