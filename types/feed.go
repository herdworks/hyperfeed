package hyperfeed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	"github.com/herdworks/hypertemplates-go/atom"
)

// ============================ Package Types =================================

// Feed represents a single RSS/Atom feed.
type Feed struct {
	ID          string      `json:"id,omitempty"`  // stable HyperFeed Feed identifier (normalized URL string)
	URL         *url.URL    `json:"url,omitempty"` // canonical feed URL (RSS/Atom endpoint)
	Updated     time.Time   `json:"updated,omitempty"`
	Language    string      `json:"language,omitempty"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	Authors     []Author    `json:"authors,omitempty"`
	Metadata    Metadata    `json:"metadata,omitempty"` // Non-standard/extension properties
	Items       []*Item     `json:"items,omitempty"`    // the most recent items for this feed
	Status      *FeedStatus `json:"status,omitempty"`   // fetch/status metadata

	Href       string  `json:"-"` // string representation of Feed.URL
	StatusJSON []byte  `json:"-"` // byte slice representation of Feed.Status (JSON)
	Bytes      []byte  `json:"-"` // the most recent contents (raw bytes) for this feed
	RawFeed    RawFeed `json:"-"` // RSS, Atom, or JSON Feed representation of the feed
}

// FeedStatus represents feed state (schedule, rate limiting, etc)
type FeedStatus struct {
	// CreatedAt indicates when the feed was first scheduled.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt indicates when the state was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// LastAttemptAt is when we most recently attempted a fetch, successful or not.
	LastAttemptAt time.Time `json:"last_attempt_at"`

	// LastSuccessAt is when we most recently fetched new content.
	LastSuccessAt time.Time `json:"last_success_at"`

	// LastErrorAt is when we most recently encountered an HTTP 4xx or 5xx error.
	LastErrorAt time.Time `json:"last_error_at"`

	// PausedUntil stores user-imposed pause windows (manual snooze).
	PausedUntil time.Time `json:"paused_until"`

	// ETag stores the value of the ETag header from the last successful fetch.
	ETag string `json:"etag"`

	// ContentLength stores the value of the Content-Length header from the last successful fetch.
	ContentLength int64 `json:"content_length"`

	// LastModified stores the value of the Last-Modified header from the last successful fetch.
	LastModified time.Time `json:"last_modified"`

	// LastStatusCode stores the HTTP status code returned on the last attempt, including 304 and error codes.
	LastStatusCode int `json:"last_status_code"`

	// LastStatusCodes stores the last 30 status codes for diagnostic purposes.
	LastStatusCodes []int `json:"last_status_codes"`

	// RateLimit captures rate limiting information supplied by the origin.
	RateLimit FeedRateLimit `json:"rate_limit"`

	// CacheControl stores parsed HTTP cache headers.
	CacheControl FeedCacheControl `json:"cache_control"`

	// Headers stores the raw headers from the most recent fetch to support future hint extraction.
	Headers http.Header `json:"headers"`
}

// FeedRateLimit captures rate limiting signals via HTTP 429/5xx status codes and related headers.
type FeedRateLimit struct {
	WaitUntil    time.Time     `json:"wait_until"`    // Retry-After time
	RetryAfter   time.Duration `json:"retry_after"`   // Retry-After seconds
	PollInterval time.Duration `json:"poll_interval"` // X-Poll-Interval in seconds
	Recent429s   int           `json:"recent_429s"`   // count of 429 responses in the last 30 requests
	Recent5xxs   int           `json:"recent_5xxs"`   // count of 5xx responses in the last 30 requests
	// TODO: implement support for X-RateLimit-Remaining headers?
	// TODO: implement support for X-RateLimit-Reset headers?
	// TODO: implement support for X-RateLimit-Window headers?
}

// FeedCacheControl normalizes cache metadata from HTTP headers.
type FeedCacheControl struct {
	Age          time.Duration `json:"age"`
	MaxAge       time.Duration `json:"max_age"`
	SharedMaxAge time.Duration `json:"shared_max_age"`
	ExpiresAt    time.Time     `json:"expires_at"`
	ETagMismatch bool          `json:"etag_mismatch"`
}

// FeedKind is a feed type/version pair (e.g., "rss/2.0")
type FeedKind string

// RawFeed represents a format-specific parsed feed (RSS/Atom/JSON Feed) before normalization. 
// Implementations should report their concrete kind and version via Kind().
type RawFeed interface {
	Kind() FeedKind                      // returns the concrete feed kind/version (e.g., rss/2.0, jsonfeed/1.1)
	GetURL() *url.URL                    // returns the feed's base URL (for resolving relative item URLs)
	SetURL(*url.URL)                     // set the raw feed's base URL (for resolving relative item URLs)
	ItemCount() int                      // returns the number of items in the feed
	ParseFeedAuthors() ([]Author, error) // parses and returns the feed's owners
	ParseItems(feedID string) ([]*Item, error)        // parses and returns the feed's items
}

// ============================ Package Variables =============================

// FeedKind enumeration
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

// ============================ Feed Methods ==================================

// Domain returns the feed's domain (host) portion of its URL.
func (feed *Feed) Domain() string {
	return normalized.Domain(feed.URL)
}

// Identifier returns the stable identifier for the feed, computing it if needed.
func (feed *Feed) Identifier() string {
	if feed.ID == "" {
		u := normalized.URL(feed.URL)
		feed.ID = normalized.Identifier(u.String())
	}
	return feed.ID
}

// PathIdentifier returns the stable identifier for the feed, computing it if needed.
func (feed *Feed) PathIdentifier() any {
	return normalized.Identifier(normalized.Path(feed.URL))
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
		if !feed.RawFeed.GetURL().IsAbs() {
			feed.RawFeed.SetURL(feed.URL) // ensure base URL is set for relative item URLs
		}
		if feed.Authors, err = feed.RawFeed.ParseFeedAuthors(); err != nil {
			return err
		}
		if feed.Items, err = feed.RawFeed.ParseItems(feed.Identifier()); err != nil {
			return err
		} else if len(feed.Items) != feed.RawFeed.ItemCount() {
			return errors.Join(ErrFeed, fmt.Errorf("item count mismatch: expected %d, got %d", feed.RawFeed.ItemCount(), len(feed.Items)))
		}
	}
	return nil
}

func (feed *Feed) RowTable() string {
	return "feeds"
}
func (feed *Feed) RowIdentifier() string {
	return "id"
}
func (feed *Feed) RowColumns() []string {
	return []string{
		"id",
		"url",
		"updated_at",
		"title",
		"description",
		"language",
		"status",
	}
}
func (feed *Feed) RowArguments() (args []any) {
	if feed.URL == nil {
		feed.URL = &url.URL{}
	}
	args = append(args, feed.Identifier())                     // id
	args = append(args, normalized.URL(feed.URL).String())     // url
	args = append(args, time.Now().UTC().Format(time.RFC3339)) // updated_at
	args = append(args, feed.Title)                            // title
	args = append(args, feed.Description)                      // description
	args = append(args, feed.Language)                         // language
	if statusBytes, err := json.Marshal(feed.Status); err == nil && statusBytes != nil {
		args = append(args, string(statusBytes)) // status
	} else {
		args = append(args, "{}") // emtpy status
	}
	return args
}
func (feed *Feed) RowUpdates() (updates [][2]string) {
	// ON CONFLICT DO UPDATE SET ...
	return [][2]string{
		{"updated_at", "excluded.updated_at"},
		{"title", "excluded.title"},
		{"description", "excluded.description"},
		{"language", "excluded.language"},
		{"status", "excluded.status"},
	}
}
func (feed *Feed) RowTargets() (targets []any) {
	targets = append(targets, &feed.ID)
	targets = append(targets, &feed.Href)
	targets = append(targets, &feed.Updated)
	targets = append(targets, &feed.Title)
	targets = append(targets, &feed.Description)
	targets = append(targets, &feed.Language)
	targets = append(targets, &feed.StatusJSON)
	return targets
}

// ============================ Status Methods ================================

// TBD if this is helpful, but it feels like an interesting experiment to optimize refresh workloads.
func (status *FeedStatus) Cooldown() (cooldown time.Duration) {
	var notmodifieds int = count(status.LastStatusCodes, 304)
	var servererrs int = count(status.LastStatusCodes, 500, 502, 503, 504)
	var toomanyreqs int = count(status.LastStatusCodes, 429)
	cooldown += time.Duration(notmodifieds) * (12 * time.Second) // 1 minute cooldown for every 5 304s
	cooldown += time.Duration(servererrs) * (30 * time.Second)   // 1 minute cooldown for every 2 5xxs
	cooldown += time.Duration(toomanyreqs) * (1 * time.Minute)   // 1 minute cooldown for every 429
	return cooldown
}

// ============================ Helper Functions ==============================

func count[T comparable](items []T, matching ...T) (result int) {
	for _, item := range items {
		if slices.Contains(matching, item) {
			result++
		}
	}
	return result
}
