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

// RawFeed.Items() compliance
func (jf *JSONFeed) ParseItems() (items []*Item) {
	for _, i := range jf.Items {
		items = append(items, jf.ParseItem(&i))
	}
	return items
}
func (jf *JSONFeed) ParseItem(jsonfeeditem *JSONItem) (item *Item) {
	item = &Item{}
	item.Added = time.Now().UTC()
	item.ID = jsonfeeditem.ID
	item.Title = jsonfeeditem.Title
	item.Summary = jsonfeeditem.Summary
	if jsonfeeditem.ContentHTML != "" {
		item.Content = jsonfeeditem.ContentHTML
	} else {
		item.Content = jsonfeeditem.ContentText
	}
	if u, err := url.Parse(jsonfeeditem.URL); err == nil && u != nil {
		item.URL = u
	}
	if t, err := time.Parse(time.RFC3339, jsonfeeditem.DatePublished); err == nil {
		item.Published = t
	}
	if t, err := time.Parse(time.RFC3339, jsonfeeditem.DateModified); err == nil {
		item.Updated = t
	}
	if u, err := url.Parse(jsonfeeditem.ExternalURL); err == nil && u != nil {
		item.Links = append(item.Links, u)
	}
	if u, err := url.Parse(jsonfeeditem.BannerImage); err == nil && u != nil {
		item.Images = append(item.Images, &Image{URL: u, Title: "Feed item banner image"})
	}
	if u, err := url.Parse(jsonfeeditem.Image); err == nil && u != nil {
		item.Images = append(item.Images, &Image{URL: u, Title: "Feed item image"})
	}
	if jsonfeeditem.Author != nil {
		item.Authors = append(item.Authors, *jf.ToAuthor(jsonfeeditem.Author))
	}
	for _, author := range jsonfeeditem.Authors {
		item.Authors = append(item.Authors, *jf.ToAuthor(&author))
	}
	return item
}

func (jf *JSONFeed) ToAuthor(jfauthor *JSONFeedAuthor) (author *Author) {
	author = &Author{Name: jfauthor.Name}
	if u, err := url.Parse(jfauthor.URL); err == nil && u != nil && u.Scheme == "mailto" {
		author.Email = u.Opaque
	}
	if u, err := url.Parse(jfauthor.Avatar); err == nil && u != nil && u.Scheme != "mailto" {
		author.URL = u
	}
	if u, err := url.Parse(jfauthor.Avatar); err == nil && u != nil {
		author.Favicon = u
	}
	return author
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
