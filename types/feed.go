package hyperfeed

import (
	"bytes"
	"net/url"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	"github.com/herdworks/hypertemplates-go/atom"
)

// Feed represents a single RSS/Atom feed.
type Feed struct {
	URL         *url.URL  `json:"url,omitempty"` // canonical feed URL (RSS/Atom endpoint)
	ID          string    `json:"id,omitempty"`  // stable HyperFeed identifier (e.g. hash of feed URL path+query)
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Language    string    `json:"language,omitempty"`
	Updated     time.Time `json:"updated,omitempty"`
	Authors     []Author  `json:"authors,omitempty"`
	Metadata    Metadata  `json:"metadata,omitempty"` // Non-standard/extension properties
	Items       []*Item   `json:"items,omitempty"`    // the most recent items for this feed
	Bytes       []byte    `json:"-"`                  // the most recent contents (raw bytes) for this feed
	RawFeed     RawFeed   `json:"-"`                  // RSS, Atom, or JSON Feed representation of the feed
}

// FeedKind is a feed type/version pair (e.g., "rss/2.0")
type FeedKind string

const (
	// Unknown or unspecified
	FeedKindUnknown      FeedKind = ""
	FeedKindRSS          FeedKind = "rss"
	FeedKindRSS_0_9_1    FeedKind = "rss/0.91"
	FeedKindRSS_0_9_2    FeedKind = "rss/0.92"
	FeedKindRSS_2_0      FeedKind = "rss/2.0"
	FeedKindAtom         FeedKind = "atom"
	FeedKindAtom_0_3     FeedKind = "atom/0.3"
	FeedKindAtom_1_0     FeedKind = "atom/1.0"
	FeedKindJSONFeed     FeedKind = "jsonfeed"
	FeedKindJSONFeed_1_0 FeedKind = "jsonfeed/1.0"
	FeedKindJSONFeed_1_1 FeedKind = "jsonfeed/1.1"
)

// RawFeed represents a format-specific parsed feed (RSS/Atom/JSON Feed)
// before normalization. Implementations should report their concrete kind
// and version via Kind().
type RawFeed interface {
	Kind() FeedKind      // returns the concrete feed kind/version (e.g., rss/2.0, jsonfeed/1.1)
	ItemCount() int      // returns the number of items in the feed
	ParseItems() []*Item // parses and returns the feed's items
}

// ============================ Feed Methods ==================================

// Domain returns the feed's domain (host) portion of its URL.
func (feed *Feed) Domain() string {
	return normalized.Domain(feed.URL)
}

// Identifier returns the stable identifier for the feed, computing it if needed.
func (feed *Feed) Identifier() string {
	if feed.ID == "" {
		feed.ID = normalized.Identifier(normalized.Path(feed.URL))
	}
	return feed.ID
}

// DetectKind inspects the provided document (JSON or XML) and returns the
// detected feed kind/version. Accepts either []byte or string input.
func (f *Feed) DetectKind() FeedKind {
	if f.Bytes == nil {
		return FeedKindUnknown
	}
	if len(bytes.TrimSpace(f.Bytes)) == 0 {
		return FeedKindUnknown // empty document
	}
	if JSONFeedDetected(f.Bytes) {
		return FeedKindJSONFeed
	}
	if RSSDetected(f.Bytes) {
		return FeedKindRSS
	}
	if atom.AtomDetected(f.Bytes) {
		return FeedKindAtom
	}
	return FeedKindUnknown
}

func (feed *Feed) Parse() (err error) {
	switch feed.DetectKind() {
	case FeedKindRSS, FeedKindRSS_0_9_1, FeedKindRSS_0_9_2, FeedKindRSS_2_0:
		if feed.RawFeed, err = ParseRSS(feed.Bytes); err != nil {
			return err
		}
	case FeedKindAtom, FeedKindAtom_0_3, FeedKindAtom_1_0:
		if feed.RawFeed, err = ParseAtom(feed.Bytes); err != nil {
			return err
		}
	case FeedKindJSONFeed, FeedKindJSONFeed_1_0, FeedKindJSONFeed_1_1:
		if feed.RawFeed, err = ParseJSONFeed(feed.Bytes); err != nil {
			return err
		}
	}
	if feed.RawFeed != nil {
		feed.Items = feed.RawFeed.ParseItems()
	}
	return nil
}
