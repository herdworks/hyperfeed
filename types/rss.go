package hyperfeed

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
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
	Links          []RSSLink     `xml:"link"`
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
	Links       []RSSLink      `xml:"link"` // some feeds use multiple <link> elements (see NYT <atom:link> namespace elements)
	Title       string         `xml:"title"`
	Description string         `xml:"description"`
	Authors     []string       `xml:"author"`
	Comments    string         `xml:"comments"`
	GUID        *RSSGUID       `xml:"guid"`
	PubDate     string         `xml:"pubDate"`
	Categories  []RSSCategory  `xml:"category"`
	Enclosures  []RSSEnclosure `xml:"enclosure"`
	Source      *RSSSource     `xml:"source"`

	// Extensions captures additional namespaced elements under <item>.
	Extensions []RSSExtension `xml:",any" json:"-"`
}

type RSSLink struct {
	Href  string `xml:",chardata"`
	Rel   string `xml:"rel,attr,omitempty"` // for Atom links
	Type  string `xml:"type,attr,omitempty"`
	Space string `xml:"xmlns,attr,omitempty"`
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

// Extension namespaces
const (
	// iTunes Podcasts: https://podcasters.apple.com/support/823-podcast-requirements
	ExtensionNamespaceItunesPodcast string = "http://www.itunes.com/dtds/podcast-1.0.dtd"

	// Google Play Podcasts
	// https://www.google.com/schemas/play-podcasts/1.0/play-podcasts.xsd
	// https://support.google.com/podcast-publishers/answer/9889544?visit_id=638961521969445342-2535738675&rd=1
	ExtensionNamespaceGooglePlayPodcasts string = "http://www.google.com/schemas/play-podcasts/1.0"

	// Media RSS
	// https://www.rssboard.org/media-rss
	ExtensionNamespaceYahooMediaRSS string = "http://search.yahoo.com/mrss/"

	// Other common extension namespaces
	ExtensionNamespaceAtom                string = "http://www.w3.org/2005/Atom"
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

// RawFeed.GetURL() compliance
func (r *RSS) GetURL() *url.URL {
	if r == nil || r.Channel.Links == nil || len(r.Channel.Links) == 0 {
		return nil
	}
	for _, link := range r.Channel.Links {
		if link.URL() != nil {
			return link.URL()
		}
	}
	return nil
}
func (r *RSS) SetURL(u *url.URL) {
	if u == nil || u.String() == "" {
		return
	}
	r.Channel.Links = append([]RSSLink{{Href: u.String()}}, r.Channel.Links...) // prepend if not already present
}

// RawFeed.ItemCount() compliance
func (r *RSS) ItemCount() int {
	return len(r.Channel.Items)
}

// RawFeed.ParseFeedAuthors() compliance
func (r *RSS) ParseFeedAuthors() (authors []Author) {
	if editor := r.ParseAuthorString(r.Channel.ManagingEditor); !editor.IsEmpty() {
		authors = append(authors, editor)
	}
	if webmaster := r.ParseAuthorString(r.Channel.Webmaster); !webmaster.IsEmpty() {
		authors = append(authors, webmaster)
	}
	authors = append(authors, r.ParseChannelExtensionAuthors()...)
	for i := range authors {
		if authors[i].URL == nil || authors[i].URL.String() == "" {
			authors[i].URL = r.GetURL()
		}
	}
	return authors
}

// RawFeed.ParseItems() compliance
func (r *RSS) ParseItems(feedID string, added time.Time) (items []*Item, err error) {
	for _, rssitem := range r.Channel.Items {
		if item, err := r.ParseItem(&rssitem); err == nil {
			item.FeedID = feedID
			item.Added = added
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
	for _, link := range rssitem.Links {
		if link.URL() != nil {
			item.URL = link.URL()
		}
	}
	if item.URL = rssitem.GetURL(); item.URL == nil {
		return nil, fmt.Errorf("FEED %q has item with no URL: %q", rssitem.GetURL(), rssitem.Title)
	} else if !item.URL.IsAbs() && r.GetURL() != nil {
		item.URL = r.GetURL().ResolveReference(item.URL) // resolve relative URLs against the feed URL
	}
	for _, author := range r.ParseAuthors(rssitem) {
		item.Authors = append(item.Authors, author)
	}
	if item.Comments, err = url.Parse(rssitem.Comments); err != nil {
		return nil, err
	}
	if item.Published, err = r.ParseDate(rssitem.PubDate, rssitem); err != nil {
		fmt.Printf("item %q pubDate error: %v\n", item.URL, err)
		return nil, err
	}
	if item.Updated.IsZero() {
		item.Updated = item.Published
	}
	for _, enclosure := range rssitem.Enclosures {
		var attachment *Attachment
		if attachment, err = r.ParseAttachment(item, rssitem, &enclosure); err != nil {
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
	if len(rssitem.Authors) == 0 {
		return r.ParseFeedAuthors()
	}
	for _, a := range rssitem.Authors {
		author := r.ParseAuthorString(a)
		if author.IsEmpty() {
			continue
		}
		author.URL = r.GetURL()
		authors = append(authors, author)
	}
	return authors
}

// <author> elements are not well-defined in RSS 2.0 and may contain just an email,
// just a name, or both in various formats.
func (r *RSS) ParseAuthorString(incoming string) Author {
	var author Author = Author{}
	var parts []string = strings.Split(strings.TrimSpace(incoming), " ")
	var emailAddress *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if emailAddress.MatchString(part) {
			author.Email = part
			continue
		}
	}
	for _, part := range parts {
		if !emailAddress.MatchString(part) {
			author.Name = strings.Trim(strings.TrimSpace(part), "()<>{}[]")
			continue
		}
	}
	return author
}
func (r *RSS) ParseChannelExtensionAuthors() (authors []Author) {
	var author Author = Author{}
	if r.Channel.Title != "" {
		author.Name = r.Channel.Title // use feed title as default author name
	}
	if ownerMetadata := GetItunesMetadata(r.Channel.Extensions, "owner"); ownerMetadata != nil {
		if name := GetItunesMetadata(ownerMetadata.Extensions, "name"); name != nil {
			author.Name = name.Text
		}
		if email := GetItunesMetadata(ownerMetadata.Extensions, "email"); email != nil {
			author.Email = email.Text
		}
	}
	if !author.IsEmpty() {
		authors = append(authors, author)
	}
	return authors
}
func (r *RSS) ParseItemExtensionAuthors(rssitem *RSSItem) (authors []Author) {
	var author Author = Author{}
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
	if !author.IsEmpty() {
		authors = append(authors, author)
	}
	return authors
}
func (r *RSS) ParseAttachment(item *Item, rssitem *RSSItem, enclosure *RSSEnclosure) (attachment *Attachment, err error) {
	attachment = &Attachment{}
	if attachment.URL, err = url.Parse(enclosure.URL); err != nil {
		return nil, err
	}
	if enclosure.Length == "" {
		enclosure.Length = "0"
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
	attachment.ItemID = item.Identifier()
	return attachment, nil
}

func (r *RSS) ParseDate(dateStr string, item *RSSItem) (t time.Time, err error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
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
	if t, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", dateStr); err == nil {
		return t.UTC(), nil
	}
	// catch non-standard ISO 8601 dates, tisk tisk
	if t, err = time.Parse(time.RFC3339, dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05-0700", dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05Z0700", dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05.000Z0700", dateStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02T15:04:05.000-0700", dateStr); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("could not parse %q date: %s", item.GetURL(), dateStr)
}

// ============================ RSSItem Methods ===============================

func (item *RSSItem) GetURL() *url.URL {
	if item == nil {
		return nil
	}
	for _, link := range item.Links {
		if link.URL() != nil {
			return link.URL()
		}
	}
	return nil
}

// ============================ RSSLink Methods ===============================

func (rsslink *RSSLink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	var atomLink AtomLink = AtomLink{}
	if rsslink == nil {
		rsslink = &RSSLink{}
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
func (rsslink *RSSLink) String() string {
	return rsslink.Href
}
func (rsslink *RSSLink) URL() *url.URL {
	if rsslink == nil || rsslink.Href == "" {
		return nil
	}
	if u, _ := url.Parse(rsslink.Href); u != nil {
		return u
	}
	return nil
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
