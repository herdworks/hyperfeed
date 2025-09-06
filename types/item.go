package hyperfeed

import (
	"net/url"
	"time"
)

// Item represents an entry within a feed.
type Item struct {
	// ID is an optional stable identifier for the item (e.g., a UUID or hash).
	ID string `json:"id,omitempty"`

	// GUID is a stable identifier provided by the source feed when available.
	GUID string `json:"guid,omitempty"`

	// URL is the canonical link for the item (permalink).
	URL *url.URL `json:"url,omitempty"`

	Title   string   `json:"title,omitempty"`
	Authors []Author `json:"authors,omitempty"`

	Published time.Time `json:"published,omitempty"`
	Updated   time.Time `json:"updated,omitempty"`

	Summary string `json:"summary,omitempty"`
	Content string `json:"content,omitempty"`

	Enclosures []Enclosure `json:"enclosures,omitempty"`
	Categories []string    `json:"categories,omitempty"`

	Image *Image `json:"image,omitempty"`

	// Metadata holds non-standard/extension Item properties
	Metadata Metadata `json:"metadata,omitempty"`
}

type Items []*Item

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

type Author struct {
	Name  string   `json:"name,omitempty"`
	URI   *url.URL `json:"uri,omitempty"`
}

// Image represents an image associated with a feed or item.
type Image struct {
	URL    *url.URL `json:"url,omitempty"`
	Title  string   `json:"title,omitempty"`
	Width  int      `json:"width,omitempty"`
	Height int      `json:"height,omitempty"`
}

// Enclosure represents a media attachment (audio, video, etc.).
type Enclosure struct {
	URL    *url.URL `json:"url,omitempty"`
	Length int64    `json:"length,omitempty"`
	Type   string   `json:"type,omitempty"`
}
