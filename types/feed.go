package hyperfeed

import (
	"net/url"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/normalization"
)

// Feed represents a single RSS/Atom feed.
type Feed struct {
	// URL is the canonical feed URL (RSS/Atom endpoint).
	URL    *url.URL `json:"url,omitempty"`
	Domain string   `json:"domain,omitempty"`
	ID     string   `json:"id,omitempty"`

	// Standard feed properties
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Language    string    `json:"language,omitempty"`
	Updated     time.Time `json:"updated,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`

	// Items contains the most recent items for this feed.
	Items Items `json:"items,omitempty"`
}

type Feeds []*Feed

// Identify resolves the feed's domain and ID.
func (f *Feed) Identify() {
	f.Domain = normalization.Domain(f.URL).String()
	f.ID = normalization.HexID(f.URL)
}

// CanonicalID returns a stable identifier for the feed, preferring ID then URL.
func (f *Feed) CanonicalID() *url.URL {
	f.Identify()
	return &url.URL{Host: f.Domain, Path: f.ID} // e.g. "example.com/abcdef1234567890"
}
