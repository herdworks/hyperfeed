package hyperfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go/types"
)

// TYPES
// =============================================================================

// Feed represents a single RSS/Atom feed.
type Feed struct {
	ID                 string        `json:"id,omitempty"`       // stable HyperFeed Feed identifier (normalized URL string)
	Kind               string        `json:"kind,omitempty"`     // feed kind (e.g. "rss", "atom", "jsonfeed", "invalid", "unknown", etc.)
	URL                *href.Href    `json:"url,omitempty"`      // canonical feed URL (RSS/Atom endpoint)
	HTMLURL            *href.Href    `json:"html_url,omitempty"` // human-readable feed URL (web page)
	Title              string        `json:"title,omitempty"`
	Description        string        `json:"description,omitempty"`
	Language           string        `json:"language,omitempty"`
	Metadata           Metadata      `json:"metadata,omitempty"` // Non-standard/extension properties
	Interval           time.Duration `json:"interval,omitempty"` // fetch interval in minutes
	UpdatedAt          time.Time     `json:"updated_at,omitempty"`
	Website            Website       `json:"website,omitempty"`             // the Website this feed is associated with
	Status             Status        `json:"status,omitempty"`              // fetch status
	SubscriptionListID string        `json:"subscriptionlist_id,omitempty"` // the SubscriptionList.ID this feed originated from

	Items []*Item `json:"-"` // the most recent items for this feed
	Bytes []byte  `json:"-"` // the most recent contents (raw bytes) for this feed
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

// VARIABLES
// =============================================================================
var (
	ErrScraper = errors.New("scraper error")
)

// METHODS
// =============================================================================

// Domain returns the feed's domain (host) portion of its URL.
func (feed *Feed) Domain() string {
	return normalized.Domain(feed.URL.URL())
}

// Identifier returns the stable identifier for the feed, computing it if needed.
func (feed *Feed) Identifier() string {
	if feed == nil {
		return ""
	}
	if feed.ID == "" {
		u := normalized.URL(feed.URL.URL())
		feed.ID = normalized.Identifier(u.String())
	}
	return feed.ID
}

// PathIdentifier returns the stable identifier for the feed, computing it if needed.
func (feed *Feed) PathIdentifier() any {
	return normalized.Identifier(normalized.Path(feed.URL.URL()))
}

func (feed *Feed) DuplicateItems() (duplicates []*Item) {
	var seen map[string]bool = map[string]bool{}
	for _, item := range feed.Items {
		if seen[item.Identifier()] {
			duplicates = append(duplicates, item)
		} else {
			seen[item.Identifier()] = true
		}
	}
	return duplicates
}

func (feed *Feed) HyperGraphEntity(ctx context.Context, timeout int) (err error) {
	var document *hypergraph.Document = &hypergraph.Document{Href: feed.HTMLURL.String()}
	if !feed.Status.LastEntityUpdateAt.IsZero() && time.Since(feed.Status.LastEntityUpdateAt) < 24*time.Hour {
		return nil // recently updated
	}
	if err = document.Download(ctx, true, nil, timeout); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("downloading document"), err)
	}
	if err = document.Scrape(ctx, true, nil, timeout); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("scraping document"), err)
	}
	feed.Website.Entity = document.Metadata.Entity
	feed.Website.ID = "" // reset website ID
	if feed.Website.Status == nil {
		feed.Website.Status = &Status{}
	}
	feed.Website.Status.UpdatedAt = time.Now().UTC()
	return nil
}

// DatabaseReadable interface methods
func (*Feed) DatabaseView() string {
	return "feeds_view"
}
func (*Feed) DatabaseViewIdentifier() string {
	return "id"
}
func (*Feed) DatabaseObject() string {
	return "feed"
}

// DatabaseWritable interface methods
func (*Feed) DatabaseTable() string {
	return "feeds"
}
func (*Feed) DatabaseTableIdentifier() string {
	return "id"
}
func (*Feed) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"kind",
		"url",
		"html_url",
		"title",
		"description",
		"language",
		"interval",
		"status",
		"subscriptionlist_id",
		"website_id",
	}
}
func (feed *Feed) DatabaseInsertValues() (values []any) {
	values = append(values, feed.Identifier())       // id
	values = append(values, feed.Kind)               // kind
	values = append(values, feed.URL.URL().String()) // url
	if feed.HTMLURL != nil {
		values = append(values, feed.HTMLURL.URL().String()) // html_url
	} else {
		values = append(values, nil) // html_url
	}
	values = append(values, feed.Title)             // title
	values = append(values, feed.Description)       // description
	values = append(values, feed.Language)          // language
	values = append(values, feed.Interval.String()) // interval
	if statusBytes, _ := json.Marshal(feed.Status); statusBytes != nil {
		values = append(values, string(statusBytes)) // status
	} else {
		values = append(values, "{}") // status
	}
	values = append(values, feed.SubscriptionListID) // subscriptionlist_id
	if len(feed.Website.HTMLURL) > 0 {
		values = append(values, feed.Website.Identifier()) // website_id
	} else {
		values = append(values, nil) // website_id
	}
	return values
}
func (feed *Feed) DatabaseUpdatesSet() []string {
	return []string{
		"url = excluded.url",
		"kind = excluded.kind",
		"html_url = excluded.html_url",
		"title = excluded.title",
		"description = excluded.description",
		"language = excluded.language",
		"interval = excluded.interval",
		"status = excluded.status",
		// never update subscriptionlist_id
		"website_id = excluded.website_id",
	}
}

// Hash computes a simple hash of the feed's significant fields for change detection.
// TODO: is this needed for anything other than testing?
func (feed *Feed) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s",
		feed.Identifier(),
		feed.Kind,
		normalized.URL(feed.URL.URL()).String(),
		normalized.URL(feed.HTMLURL.URL()).String(),
		feed.UpdatedAt.UTC().Format(time.RFC3339),
		feed.Title,
		feed.Description,
		feed.Language,
	)
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
