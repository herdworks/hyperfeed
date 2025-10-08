package hyperfeed

import (
	"encoding/json"
	"net/url"
	"time"
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

// RawFeed.ItemCount() compliance
func (jf *JSONFeed) ItemCount() int {
	return len(jf.Items)
}

// RawFeed.ParseFeedAuthors() compliance
func (jf *JSONFeed) ParseFeedAuthors() (authors []Author, err error) {
	if jf.Author != nil {
		authors = append(authors, *jf.ParseAuthor(jf.Author))
	}
	for _, author := range jf.Authors {
		authors = append(authors, *jf.ParseAuthor(&author))
	}

	// ensure at least one author with URL matching JSONFeed.HomePageURL
	for _, author := range authors {
		if author.URL.String() == jf.HomePageURL {
			return authors, nil // if author URL matches feed home page URL, return
		}
	}
	if u, err := url.Parse(jf.HomePageURL); err == nil && u != nil {
		authors = append([]Author{{Name: jf.Title, URL: u}}, authors...) // prepend JSONFeed.HomePageURL author
	}
	return authors, nil
}

// RawFeed.Items() compliance
func (jf *JSONFeed) ParseItems() (items []*Item, err error) {
	for _, i := range jf.Items {
		if item, err := jf.ParseItem(&i); err == nil {
			items = append(items, item)
		} else {
			return items, err
		}
	}
	return items, nil
}
func (jf *JSONFeed) ParseItem(jfitem *JSONItem) (item *Item, err error) {
	item = &Item{}
	item.Added = time.Now().UTC()
	item.ID = jfitem.ID
	item.Title = jfitem.Title
	item.Summary = jfitem.Summary
	if jfitem.ContentHTML != "" {
		item.Content = jfitem.ContentHTML
	} else {
		item.Content = jfitem.ContentText
	}
	if u, err := url.Parse(jfitem.URL); err == nil && u != nil {
		item.URL = u
	}
	if t, err := time.Parse(time.RFC3339, jfitem.DatePublished); err == nil {
		item.Published = t
	}
	if t, err := time.Parse(time.RFC3339, jfitem.DateModified); err == nil {
		item.Updated = t
	}
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
	if jfitem.Author != nil {
		authors = append(authors, *jf.ParseAuthor(jfitem.Author))
	}
	for _, author := range jfitem.Authors {
		authors = append(authors, *jf.ParseAuthor(&author))
	}
	return authors
}
func (jf *JSONFeed) ParseAuthor(jfauthor *JSONFeedAuthor) (author *Author) {
	if jfauthor == nil {
		return nil
	} else {
		author = &Author{Name: jfauthor.Name}
	}
	if u, err := url.Parse(jf.HomePageURL); err == nil {
		author.URL = u // default to feed home page URL
	}
	if u, err := url.Parse(jfauthor.URL); err == nil && u != nil && u.Host != "" && u.Scheme != "mailto" {
		author.URL = u // overwrite default feed home page URL
	} else if u.Scheme == "mailto" {
		author.Email = u.Opaque
	}
	if u, err := url.Parse(jfauthor.Avatar); err == nil && u != nil {
		author.Favicon = u
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
	if attachment.URL, err = url.Parse(jfattachment.URL); err != nil {
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
