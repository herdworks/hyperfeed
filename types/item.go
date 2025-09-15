package hyperfeed

import (
	"net/url"
	"time"
)

// Item represents an entry within a feed.
type Item struct {
	// ID is the stable HyperFeed Item identifier (e.g., a UUID or hash).
	ID string `json:"id,omitempty"`

	// GUID is a stable identifier provided by the source feed when available.
	GUID string `json:"guid,omitempty"`

	// URL is the canonical link for the item (permalink).
	URL *url.URL `json:"url,omitempty"`

	Title   string   `json:"title,omitempty"`
	Authors []Author `json:"authors,omitempty"`

	Published time.Time `json:"published,omitempty"` // created timestamp
	Updated   time.Time `json:"updated,omitempty"`   // modified timestamp

	Summary string `json:"summary,omitempty"`
	Content string `json:"content,omitempty"`

	Attachments []Attachment `json:"attachments,omitempty"`
	Categories  []string     `json:"categories,omitempty"`

	Image *Image `json:"image,omitempty"`

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

func (i *Item) Valid() bool {
	if i == nil {
		return false
	}
	if len(i.CanonicalID()) == 0 {
		return false
	}
	return true
}

// CanonicalID returns a stable identifier for the item, preferring ID, then GUID, then URL.
func (i *Item) CanonicalID() string {
	if i == nil {
		return ""
	}
	if i.ID != "" {
		return i.ID
	}
	if i.GUID != "" {
		return i.GUID
	}
	if i.URL != nil {
		return i.URL.String()
	}
	return ""
}
