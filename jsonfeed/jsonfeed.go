package jsonfeed

import (
	"encoding/json"
	"net/url"
	"regexp"

	"github.com/herdworks/hyperfeed/href"
)

// TYPES
// =============================================================================

// jsonfeed.Feed represents a JSONFeed document as defined by https://jsonfeed.org/version/1.1
type Feed struct {
	Version     string   `json:"version"`
	Title       string   `json:"title,omitempty"`
	FeedURL     string   `json:"feed_url,omitempty"`
	HomePageURL string   `json:"home_page_url,omitempty"`
	Description string   `json:"description,omitempty"`
	UserComment string   `json:"user_comment,omitempty"`
	NextURL     string   `json:"next_url,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Favicon     string   `json:"favicon,omitempty"`
	Author      *Author  `json:"author,omitempty"`
	Authors     []Author `json:"authors,omitempty"`
	Language    string   `json:"language,omitempty"`
	Expired     bool     `json:"expired,omitempty"`
	Hubs        []Hub    `json:"hubs,omitempty"`
	Items       []Item   `json:"items,omitempty"`

	Origin *url.URL `json:"-"`
}

// Author represents an author in a JSON Feed document.
type Author struct {
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// Hub represents a hub in a JSON Feed document.
type Hub struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Attachment represents an attachment on a JSON Feed item.
type Attachment struct {
	URL               string `json:"url,omitempty"`
	MimeType          string `json:"mime_type,omitempty"`
	Title             string `json:"title,omitempty"`
	SizeInBytes       int64  `json:"size_in_bytes,omitempty"`
	DurationInSeconds int64  `json:"duration_in_seconds,omitempty"`
}

// Item represents an individual entry in a JSON Feed document.
type Item struct {
	ID            string       `json:"id"`
	URL           string       `json:"url,omitempty"`
	ExternalURL   string       `json:"external_url,omitempty"`
	Title         string       `json:"title,omitempty"`
	ContentHTML   string       `json:"content_html,omitempty"`
	ContentText   string       `json:"content_text,omitempty"`
	Summary       string       `json:"summary,omitempty"`
	Image         string       `json:"image,omitempty"`
	BannerImage   string       `json:"banner_image,omitempty"`
	DatePublished string       `json:"date_published,omitempty"`
	DateModified  string       `json:"date_modified,omitempty"`
	Author        *Author      `json:"author,omitempty"`
	Authors       []Author     `json:"authors,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
	Language      string       `json:"language,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
}

// Variables
// =============================================================================
var (
	htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
)

// METHODS
// =============================================================================
func (feed *Feed) FeedVersion() string {
	switch feed.Version {
	case "https://jsonfeed.org/version/1.0":
		return "1.0"
	case "https://jsonfeed.org/version/1.1":
		return "1.1"
	default:
		return "unknown"
	}
}

// Entry.ExternalURLs() returns a slice of external URLs found in the entry's links.
func (item *Item) ExternalURLs(base *url.URL) (urls []*url.URL) {
	if h, err := href.Parse(item.ExternalURL); err == nil && h != nil && h.Host != "" && !h.SameOrigin(base) {
		urls = append(urls, h.URL())
	}
	return urls
}

// FUNCTIONS
// =============================================================================
func Detected(doc []byte) bool {
	var tmp struct {
		Version string `json:"version"`
	}
	return json.Unmarshal(doc, &tmp) == nil && tmp.Version != ""
}

func Parse(doc []byte) (jf *Feed, err error) {
	if err = json.Unmarshal(doc, &jf); err != nil {
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
	return jf, nil
}

func HTML(text string) string {
	if text != "" && !htmlTagPattern.MatchString(text) {
		return "<p>" + text + "</p>"
	}
	return text
}
