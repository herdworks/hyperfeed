package hyperfeed

import (
	"bytes"
	"net/url"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/normalization"
	"github.com/herdworks/hypertemplates-go/atom"
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
	Authors     []Author  `json:"authors,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`

	// Items contains the most recent items for this feed.
	Items []*Item `json:"items,omitempty"`

	// RawFeed holds the RSS, Atom, or JSON Feed representation of the feed.
	RawFeed RawFeed `json:"-"`
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

// DetectKind inspects the provided document (JSON or XML) and returns the
// detected feed kind/version. Accepts either []byte or string input.
func (f *Feed) DetectKind(doc any) FeedKind {
	var b []byte
	switch v := any(doc).(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return FeedKindUnknown
	}

	// empty document detection
	if len(bytes.TrimSpace(b)) == 0 {
		return FeedKindUnknown
	}

	// JSONFeed detection
	if JSONFeedDetected(b) {
		return FeedKindJSONFeed
	}

	// RSS detection
	if RSSDetected(b) {
		return FeedKindRSS
	}

	// Atom detection
	if atom.AtomDetected(b) {
		return FeedKindAtom
	}

	return FeedKindUnknown
}

// FeedKind enumerates known feed type/version pairs (e.g., "rss/2.0").
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
	// Kind returns the concrete feed kind/version (e.g., rss/2.0, jsonfeed/1.1).
	Kind() FeedKind
	ItemCount() int
}

func (feed *Feed) AutoParse(doc []byte) (err error) {
	return feed.Parse(feed.DetectKind(doc), doc)
}

func (feed *Feed) Parse(kind FeedKind, doc []byte) (err error) {
	switch kind {
	case FeedKindRSS, FeedKindRSS_0_9_1, FeedKindRSS_0_9_2, FeedKindRSS_2_0:
		rawfeed, err := ParseRSS(doc)
		if err != nil {
			return err
		}
		feed.RawFeed = rawfeed
		return nil
	case FeedKindAtom, FeedKindAtom_0_3, FeedKindAtom_1_0:
		rawfeed, err := ParseAtom(doc)
		if err != nil {
			return err
		}
		feed.RawFeed = rawfeed
		return nil
	case FeedKindJSONFeed, FeedKindJSONFeed_1_0, FeedKindJSONFeed_1_1:
		rawfeed, err := ParseJSONFeed(doc)
		if err != nil {
			return err
		}
		feed.RawFeed = rawfeed
		return nil
	}
	return nil
}
