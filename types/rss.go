package hyperfeed

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"time"

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

	Extensions []RSSExtension `xml:",any" json:"-"`
}

type RSSItem struct {
	Link        string         `xml:"link"`
	Title       string         `xml:"title"`
	Description string         `xml:"description"`
	Author      string         `xml:"author"`
	Comments    string         `xml:"comments"`
	GUID        *RSSGUID       `xml:"guid"`
	PubDate     string         `xml:"pubDate"`
	Categories  []RSSCategory  `xml:"category"`
	Enclosures  []RSSEnclosure `xml:"enclosure"`
	Source      *RSSSource     `xml:"source"`

	// Extensions captures additional namespaced elements under <item>.
	Extensions []RSSExtension `xml:",any" json:"-"`
}

// RSSExtension preserves any XML element outside the core spec during parsing.
// It captures the qualified name, attributes, character data, and inner XML.
type RSSExtension struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `json:"-"`
	Text     string     `json:"-"`
	InnerXML string     `json:"-"`

	Extensions []RSSExtension `xml:",any" json:"-"`
}

type RSSEnclosure struct {
	URL    string `xml:"url,attr"`    // URL of the enclosure
	Length string `xml:"length,attr"` // Size in bytes
	Type   string `xml:"type,attr"`   // MIME type
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

// ============================ Variables =====================================

const (
	ExtensionNamespaceItunesPodcast       string = "http://www.itunes.com/dtds/podcast-1.0.dtd"
	ExtensionNamespaceYahooMediaRSS       string = "http://search.yahoo.com/mrss/"
	ExtensionNamespaceRSSContent          string = "http://purl.org/rss/1.0/modules/content/"
	ExtensionNamespaceDublinCoreTerms     string = "http://purl.org/dc/terms/"
	ExtensionNamespaceDublicCoreMetadata  string = "http://purl.org/dc/elements/1.1/"
	ExtensionNamespaceScriptingNewsSource string = "http://source.scripting.com/"
)

// ============================ RSS Methods ===================================

// RawFeed.Kind() compliance
func (r *RSS) Kind() FeedKind {
	return FeedKindRSS_2_0
}

// RawFeed.ItemCount() compliance
func (r *RSS) ItemCount() int {
	return len(r.Channel.Items)
}

// RawFeed.ParseFeedAuthors() compliance
func (r *RSS) ParseFeedAuthors() (authors []Author, err error) {
	var author Author = Author{}
	var href string
	if r.Channel.Link != "" {
		href = r.Channel.Link
	}
	if r.Channel.Title != "" {
		author.Name = r.Channel.Title
	}
	if owner := GetItunesMetadata(r.Channel.Extensions, "owner"); owner != nil {
		if name := GetItunesMetadata(owner.Extensions, "name"); name != nil {
			author.Name = name.Text
		}
		if email := GetItunesMetadata(owner.Extensions, "email"); email != nil {
			author.Email = email.Text
		}
	}
	if u, err := url.Parse(href); err == nil && u != nil {
		author.URL = u
	}
	authors = append(authors, author)
	return authors, nil
}

// RawFeed.ParseItems() compliance
func (r *RSS) ParseItems() (items []*Item, err error) {
	for _, rssitem := range r.Channel.Items {
		if item, err := r.ParseItem(&rssitem); err == nil {
			items = append(items, item)
		} else {
			return items, err
		}
	}
	return items, nil
}
func (r *RSS) ParseItem(rssitem *RSSItem) (item *Item, err error) {
	item = &Item{}
	item.Added = time.Now().UTC()
	item.Title = rssitem.Title
	item.Summary = rssitem.Description
	if item.URL, err = url.Parse(rssitem.Link); err != nil {
		return nil, err
	}
	if rssitem.GUID != nil {
		item.ID = rssitem.GUID.Value
	}
	for _, author := range r.ParseAuthors(rssitem) {
		item.Authors = append(item.Authors, author)
	}
	if item.Comments, err = url.Parse(rssitem.Comments); err != nil {
		return nil, err
	}
	if item.Published, err = r.ParseDate(rssitem.PubDate); err != nil {
		return nil, err
	}
	for _, enclosure := range rssitem.Enclosures {
		var attachment *Attachment
		if attachment, err = r.ParseAttachment(rssitem, &enclosure); err != nil {
			return nil, err
		}
		item.Attachments = append(item.Attachments, *attachment)
	}
	for _, category := range rssitem.Categories {
		item.Categories = append(item.Categories, category.Text)
	}
	return item, nil
}
func (r *RSS) ParseAuthors(rssitem *RSSItem) (authors []Author) {
	var author Author = Author{}
	var href string
	if r.Channel.Link != "" {
		href = r.Channel.Link
	}
	if r.Channel.Title != "" {
		author.Name = r.Channel.Title
	}
	if ownerMetadata := GetItunesMetadata(r.Channel.Extensions, "owner"); ownerMetadata != nil {
		if name := GetItunesMetadata(ownerMetadata.Extensions, "name"); name != nil {
			author.Name = name.Text
		}
		if email := GetItunesMetadata(ownerMetadata.Extensions, "email"); email != nil {
			author.Email = email.Text
		}
	}
	if authorMetadata := GetItunesMetadata(rssitem.Extensions, "author"); authorMetadata != nil {
		author.Name = authorMetadata.Text
	}
	if u, err := url.Parse(href); err == nil && u != nil {
		author.URL = u
	}
	authors = append(authors, author)
	return authors
}
func (r *RSS) ParseAttachment(rssitem *RSSItem, enclosure *RSSEnclosure) (attachment *Attachment, err error) {
	attachment = &Attachment{}
	if attachment.URL, err = url.Parse(enclosure.URL); err != nil {
		return nil, err
	}
	if attachment.Size, err = strconv.ParseInt(enclosure.Length, 10, 64); err != nil {
		return nil, err
	}
	if duration := GetItunesMetadata(rssitem.Extensions, "duration"); duration != nil {
		if v, err := time.Parse("15:04:05", duration.Text); err == nil {
			attachment.Duration = int64(v.Hour()*3600 + v.Minute()*60 + v.Second())
		}
	}
	attachment.Mimetype = enclosure.Type
	if rssitem.GUID != nil {
		attachment.ItemID = rssitem.GUID.Value
	}
	return attachment, nil
}

func (r *RSS) ParseDate(dateStr string) (t time.Time, err error) {
	if t, err = time.Parse(time.RFC1123, dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse(time.RFC1123Z, dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse(time.RFC822, dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse(time.RFC822Z, dateStr); err == nil {
		return t.UTC(), nil
	}
	// catch dates without zero-padded day
	if t, err = time.Parse("Mon, 2 Jan 2006 15:04:05 MST", dateStr); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

// ============================ Extension Methods =============================

// UnmarshalXML captures attributes, text, inner XML, and any nested extensions.
func (e *RSSExtension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var payload struct {
		Text  string         `xml:",chardata"`
		Inner string         `xml:",innerxml"`
		Any   []RSSExtension `xml:",any"`
	}
	if err := d.DecodeElement(&payload, &start); err != nil {
		return err
	}
	e.XMLName = start.Name
	e.Attrs = append(e.Attrs[:0], start.Attr...)
	e.Text = payload.Text
	e.InnerXML = payload.Inner
	e.Extensions = append(e.Extensions[:0], payload.Any...)
	return nil
}

// ============================ Helper Functions ==============================

// RSSDetected returns true if the document appears to be RSS.
func RSSDetected(doc []byte) bool {
	b := bytes.TrimSpace(doc)
	return bytes.Contains(b, []byte("<rss"))
}

// ParseRSS parses raw RSS XML into an RSS struct.
func ParseRSS(doc []byte) (rss *RSS, err error) {
	var decoder *xml.Decoder = xml.NewDecoder(bytes.NewReader(doc))
	decoder.CharsetReader = charset.NewReaderLabel
	if err = decoder.Decode(&rss); err != nil {
		return nil, err
	}
	return rss, nil
}

func GetExtensionMetadata(extensions []RSSExtension, namespace string, name string) *RSSExtension {
	for _, ext := range extensions {
		if ext.XMLName.Space == namespace && ext.XMLName.Local == name {
			return &ext
		}
	}
	return nil
}

func GetItunesMetadata(extensions []RSSExtension, name string) *RSSExtension {
	return GetExtensionMetadata(extensions, ExtensionNamespaceItunesPodcast, name)
}

func GetYahooMediaRSSMetadata(extensions []RSSExtension, name string) *RSSExtension {
	return GetExtensionMetadata(extensions, ExtensionNamespaceYahooMediaRSS, name)
}

func GetRSSContentMetadata(extensions []RSSExtension, name string) *RSSExtension {
	return GetExtensionMetadata(extensions, ExtensionNamespaceRSSContent, name)
}
