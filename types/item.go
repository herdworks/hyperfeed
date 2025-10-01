package hyperfeed

import (
	"net/url"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
)

// Item represents an entry within a feed.
type Item struct {
	// URL is the canonical link for the item (permalink).
	URL *url.URL `json:"url,omitempty"`

	// GUID is a stable identifier provided by the source feed when available.
	GUID string `json:"guid,omitempty"`

	// ID is the stable HyperFeed Item identifier (e.g., a UUID or hash).
	ID string `json:"id,omitempty"`

	Title   string   `json:"title,omitempty"`
	Authors []Author `json:"authors,omitempty"`

	Published time.Time `json:"published,omitempty"` // created timestamp
	Updated   time.Time `json:"updated,omitempty"`   // modified timestamp

	Summary     string       `json:"summary,omitempty"`
	Content     string       `json:"content,omitempty"`
	Images      []*Image     `json:"images,omitempty"` // associated images (e.g. cover images, thumbnails)
	Links       []*url.URL   `json:"links,omitempty"`
	Categories  []string     `json:"categories,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"` // RSS enclosures, JSONFeed attachments

	// Comments is a URL pointing to a comments page for the item.
	Comments *url.URL `json:"comments,omitempty"`

	// Metadata holds non-standard/extension Item properties
	Metadata Metadata `json:"metadata,omitempty"`
}

type Author struct {
	Name   string   `json:"name,omitempty"`
	URI    *url.URL `json:"uri,omitempty"`
	URL    *url.URL `json:"url,omitempty"`
	Email  string   `json:"email,omitempty"`
	Avatar *url.URL `json:"avatar,omitempty"`
}

// Image represents an image associated with a feed or item.
type Image struct {
	URL    *url.URL `json:"url,omitempty"`
	Title  string   `json:"title,omitempty"`
	Width  int      `json:"width,omitempty"`
	Height int      `json:"height,omitempty"`
}

func (i *Item) Ready() bool {
	if i == nil {
		return false
	}
	if i.URL == nil {
		return false
	}
	return true
}

func (item *Item) Identifier() string {
	if item.ID == "" {
		u := normalized.URL(item.URL)
		item.ID = normalized.Identifier(u.String())
	}
	return item.ID
}
