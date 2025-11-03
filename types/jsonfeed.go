package hyperfeed

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	href "github.com/herdworks/hyperfeed-go/types/href"
)

// JSONFeed represents a JSON Feed document as defined by https://jsonfeed.org/version/1.1
type JSONFeed struct {
	Version     string           `json:"version"`
	Title       string           `json:"title,omitempty"`
	FeedURL     string           `json:"feed_url,omitempty"`
	HomePageURL string           `json:"home_page_url,omitempty"`
	Description string           `json:"description,omitempty"`
	UserComment string           `json:"user_comment,omitempty"`
	NextURL     string           `json:"next_url,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Favicon     string           `json:"favicon,omitempty"`
	Author      *JSONFeedAuthor  `json:"author,omitempty"`
	Authors     []JSONFeedAuthor `json:"authors,omitempty"`
	Language    string           `json:"language,omitempty"`
	Expired     bool             `json:"expired,omitempty"`
	Hubs        []JSONFeedHub    `json:"hubs,omitempty"`
	Items       []JSONItem       `json:"items,omitempty"`
}

// JSONFeedAuthor represents an author in a JSON Feed document.
type JSONFeedAuthor struct {
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// JSONFeedHub represents a hub in a JSON Feed document.
type JSONFeedHub struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// JSONFeedAttachment represents an attachment on a JSON Feed item.
type JSONFeedAttachment struct {
	URL              string `json:"url,omitempty"`
	MimeType         string `json:"mime_type,omitempty"`
	Title            string `json:"title,omitempty"`
	SizeInBytes      int64  `json:"size_in_bytes,omitempty"`
	DurationInSecond int64  `json:"duration_in_seconds,omitempty"`
}

// JSONItem represents an individual entry in a JSON Feed document.
type JSONItem struct {
	ID            string               `json:"id"`
	URL           string               `json:"url,omitempty"`
	ExternalURL   string               `json:"external_url,omitempty"`
	Title         string               `json:"title,omitempty"`
	ContentHTML   string               `json:"content_html,omitempty"`
	ContentText   string               `json:"content_text,omitempty"`
	Summary       string               `json:"summary,omitempty"`
	Image         string               `json:"image,omitempty"`
	BannerImage   string               `json:"banner_image,omitempty"`
	DatePublished string               `json:"date_published,omitempty"`
	DateModified  string               `json:"date_modified,omitempty"`
	Author        *JSONFeedAuthor      `json:"author,omitempty"`
	Authors       []JSONFeedAuthor     `json:"authors,omitempty"`
	Tags          []string             `json:"tags,omitempty"`
	Language      string               `json:"language,omitempty"`
	Attachments   []JSONFeedAttachment `json:"attachments,omitempty"`
}

// ============================ JSONFeed Methods ==============================

// RawFeed.Kind() compliance
func (jf JSONFeed) Kind() FeedKind {
	switch jf.Version {
	case "https://jsonfeed.org/version/1.0":
		return FeedKindJSONFeed_1_0
	case "https://jsonfeed.org/version/1.1":
		return FeedKindJSONFeed_1_1
	default:
		return FeedKindUnknown
	}
}

// RawFeed.GetURL() compliance
func (jf *JSONFeed) GetURL() (baseurl *url.URL) {
	var err error
	if baseurl, err = url.Parse(jf.FeedURL); err != nil || baseurl == nil || baseurl.Host == "" {
		if baseurl, err = url.Parse(jf.HomePageURL); err != nil {
			return nil
		}
	}
	return baseurl
}
func (jf *JSONFeed) SetURL(u *url.URL) {
	if u == nil {
		return
	}
	if jf.FeedURL == "" {
		jf.FeedURL = u.String()
	}
}

// RawFeed.ItemCount() compliance
func (jf *JSONFeed) ItemCount() int {
	return len(jf.Items)
}

// RawFeed.ParseFeedAuthors() compliance
func (jf *JSONFeed) ParseFeedAuthors() (authors []Author) {
	if jf.Author != nil {
		authors = append(authors, *jf.ParseAuthor(jf.Author))
	}
	for _, author := range jf.Authors {
		authors = append(authors, *jf.ParseAuthor(&author))
	}

	// ensure at least one author with URL matching JSONFeed.HomePageURL
	for _, author := range authors {
		if author.Href.String() == jf.HomePageURL {
			return authors // if author URL matches feed home page URL, return
		}
	}
	if h, err := href.Parse(jf.HomePageURL); err == nil && h != nil {
		authors = append([]Author{{Name: jf.Title, Href: h}}, authors...) // prepend JSONFeed.HomePageURL author
	}
	return authors
}

// RawFeed.Items() compliance
func (jf *JSONFeed) ParseItems(feedID string, added time.Time) (items []*Item, err error) {
	for _, i := range jf.Items {
		if item, err := jf.ParseItem(&i); err == nil {
			item.FeedID = feedID
			item.AddedAt = added
			item.UpdatedAt = added
			items = append(items, item)
		} else {
			return items, err
		}
	}
	return items, nil
}
func (jf *JSONFeed) ParseItem(jfitem *JSONItem) (item *Item, err error) {
	var published, modified time.Time
	item = &Item{}
	item.Title = jfitem.Title
	item.Summary = jfitem.Summary
	if jfitem.ContentHTML != "" {
		item.Content = jfitem.ContentHTML
	} else {
		item.Content = jfitem.ContentText
	}
	if item.URL, err = url.Parse(jfitem.URL); err != nil {
		return nil, err
	} else if !item.URL.IsAbs() && jf.GetURL() != nil {
		item.URL = jf.GetURL().ResolveReference(item.URL) // resolve relative URLs against the feed URL
	}
	if published, err = time.Parse(time.RFC3339, jfitem.DatePublished); jfitem.DatePublished != "" && err != nil {
		fmt.Printf("error parsing %q published date %q: %v\n", jfitem.URL, jfitem.DatePublished, err)
		published = time.Time{}
	}
	if modified, err = time.Parse(time.RFC3339, jfitem.DateModified); jfitem.DateModified != "" && err != nil {
		fmt.Printf("error parsing %q modified date %q: %v\n", jfitem.URL, jfitem.DateModified, err)
		modified = time.Time{}
	}
	item.PublishedAt = earliest(published, modified)
	item.ModifiedAt = latest(published, modified)
	item.AddedAt = time.Now().UTC()
	if u, err := url.Parse(jfitem.ExternalURL); err == nil && u != nil {
		item.Links = append(item.Links, u)
	}
	if u, err := url.Parse(jfitem.BannerImage); err == nil && u != nil {
		item.Images = append(item.Images, &Image{URL: u, Title: "Feed item banner image"})
	}
	if u, err := url.Parse(jfitem.Image); err == nil && u != nil {
		item.Images = append(item.Images, &Image{URL: u, Title: "Feed item image"})
	}
	item.Authors = jf.ParseAuthors(jfitem)
	item.Attachments = jf.ParseAttachments(jfitem)
	return item, nil
}
func (jf *JSONFeed) ParseAuthors(jfitem *JSONItem) (authors []Author) {
	if jfitem.Author == nil {
		authors = append(authors, *jf.ParseAuthor(&JSONFeedAuthor{}))
	} else {
		authors = append(authors, *jf.ParseAuthor(jfitem.Author))
	}
	for _, author := range jfitem.Authors {
		authors = append(authors, *jf.ParseAuthor(&author))
	}
	return authors
}
func (jf *JSONFeed) ParseAuthor(jfauthor *JSONFeedAuthor) (author *Author) {
	if jfauthor == nil && jf.Author != nil {
		author = &Author{Name: jf.Author.Name}
	} else if jfauthor == nil && jf.Author == nil {
		author = &Author{Name: jf.Title} // default to feed title
	} else {
		author = &Author{Name: jfauthor.Name}
	}
	if h, err := href.Parse(jf.HomePageURL); err == nil {
		author.Href = h // default to feed home page URL
	}
	if h, err := href.Parse(jfauthor.URL); err == nil && h != nil && h.Host != "" && h.Scheme != "mailto" {
		author.Href = h // overwrite default feed home page URL
	} else if h.Scheme == "mailto" {
		author.Email = h.Opaque
	}
	if h, err := href.Parse(jfauthor.Avatar); err == nil && h != nil {
		author.Favicon = h
	}
	return author
}
func (jf *JSONFeed) ParseAttachments(jfitem *JSONItem) (attachments []Attachment) {
	for _, attachment := range jfitem.Attachments {
		if a, err := jf.ParseAttachment(jfitem, &attachment); err == nil && a != nil {
			attachments = append(attachments, *a)
		}
	}
	return attachments
}
func (jf *JSONFeed) ParseAttachment(jfitem *JSONItem, jfattachment *JSONFeedAttachment) (attachment *Attachment, err error) {
	attachment = &Attachment{}
	if attachment.URL, err = href.Parse(jfattachment.URL); err != nil {
		return nil, err
	}
	attachment.Size = jfattachment.SizeInBytes
	attachment.Mimetype = jfattachment.MimeType
	attachment.Title = jfattachment.Title
	attachment.Duration = jfattachment.DurationInSecond
	attachment.ItemID = jfitem.ID
	return attachment, nil
}

// ============================ Helper Functions ==============================

// JSONFeedDetected returns true if the document appears to be JSON Feed.
func JSONFeedDetected(doc []byte) bool {
	var tmp struct {
		Version string `json:"version"`
	}
	return json.Unmarshal(doc, &tmp) == nil && tmp.Version != ""
}

// ParseJSONFeed converts a JSON Feed document into a Feed and Items.
func ParseJSONFeed(doc []byte) (*JSONFeed, error) {
	var jf JSONFeed
	if err := json.Unmarshal(doc, &jf); err != nil {
		return nil, err
	}
	if len(jf.Authors) == 0 && jf.Author != nil {
		jf.Authors = append(jf.Authors, *jf.Author)
	}
	for _, item := range jf.Items {
		if len(item.Authors) == 0 && item.Author != nil {
			item.Authors = append(item.Authors, *item.Author)
		}
	}
	return &jf, nil
}
