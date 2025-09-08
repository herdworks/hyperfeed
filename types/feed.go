package hyperfeed

import (
	"net/url"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/normalization"
)

// FeedKind enumerates known feed type/version pairs (e.g., "rss/2.0").
type FeedKind string

const (
	// Unknown or unspecified
	FeedKindUnknown      FeedKind = ""
	FeedKindRSS_0_9_1    FeedKind = "rss/0.91"
	FeedKindRSS_0_9_2    FeedKind = "rss/0.92"
	FeedKindRSS_2_0      FeedKind = "rss/2.0"
	FeedKindAtom_0_3     FeedKind = "atom/0.3"
	FeedKindAtom_1_0     FeedKind = "atom/1.0"
	FeedKindJSONFeed_1_0 FeedKind = "jsonfeed/1.0"
	FeedKindJSONFeed_1_1 FeedKind = "jsonfeed/1.1"
)

// Feed represents a single RSS/Atom feed.
type Feed struct {
	// URL is the canonical feed URL (RSS/Atom endpoint).
	URL    *url.URL `json:"url,omitempty"`
	Domain string   `json:"domain,omitempty"`
	ID     string   `json:"id,omitempty"`
	Kind   FeedKind `json:"kind,omitempty"`

	// Standard feed properties
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Language    string    `json:"language,omitempty"`
	Updated     time.Time `json:"updated,omitempty"`
	Authors     []Author  `json:"authors,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`

	// Items contains the most recent items for this feed.
	Items []*Item `json:"items,omitempty"`
}

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
