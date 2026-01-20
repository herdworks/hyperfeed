package hyperfeed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	*DatabaseRecord

	ID                 string        `json:"id,omitempty"`       // stable HyperFeed Feed identifier (normalized URL string)
	Kind               string        `json:"kind"`               // feed kind (e.g. "rss", "atom", "jsonfeed", "invalid", "unknown", etc.)
	URL                *href.Href    `json:"url,omitempty"`      // canonical feed URL (RSS/Atom endpoint)
	HTMLURL            *href.Href    `json:"html_url,omitempty"` // human-readable feed URL (web page)
	Title              string        `json:"title,omitempty"`
	Description        string        `json:"description,omitempty"`
	Language           string        `json:"language,omitempty"`
	IsPodcast          bool          `json:"is_podcast,omitempty"`
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
		return normalized.Identifier("") // the unknown identifier
	}
	if feed.ID == "" {
		id := normalized.URL(feed.URL.URL())
		feed.ID = normalized.Identifier(id.String())
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

func (feed *Feed) WebsiteMetadata(ctx context.Context, timeout int, w io.Writer) (err error) {
	var document *hypergraph.Document = &hypergraph.Document{Href: feed.HTMLURL.String()}
	if w == nil {
		w = os.Stdout
	}
	if !feed.Status.LastEntityUpdateAt.IsZero() && time.Since(feed.Status.LastEntityUpdateAt) < 24*time.Hour {
		return nil // recently updated
	}
	if err = document.Download(ctx, true, nil, timeout, w); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("downloading document"), err)
	}
	if err = document.Scrape(ctx, true, nil, timeout, w); err != nil {
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

// DatabaseRecord methods
func (*Feed) DatabaseObject() string {
	return "feed"
}
func (*Feed) DatabaseTable() string {
	return "feeds"
}
func (*Feed) DatabaseView() string {
	return "feeds_view"
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
		"is_podcast",
		"interval",
		"status",
		"subscriptionlist_id",
		"website_id",
	}
}
func (feed *Feed) DatabaseInsertValues() (values []any) {
	values = append(values, sql.Named("id", feed.Identifier()))        // id
	values = append(values, sql.Named("kind", feed.Kind))              // kind
	values = append(values, sql.Named("url", feed.URL.URL().String())) // url
	if feed.HTMLURL != nil {
		values = append(values, sql.Named("html_url", feed.HTMLURL.URL().String())) // html_url
	} else {
		values = append(values, sql.Named("html_url", nil)) // html_url
	}
	values = append(values, sql.Named("title", feed.Title))                // title
	values = append(values, sql.Named("description", feed.Description))    // description
	values = append(values, sql.Named("language", feed.Language))          // language
	values = append(values, sql.Named("is_podcast", feed.IsPodcast))       // is_podcast
	values = append(values, sql.Named("interval", feed.Interval.String())) // interval
	if statusBytes, _ := json.Marshal(feed.Status); statusBytes != nil {
		values = append(values, sql.Named("status", string(statusBytes))) // status
	} else {
		values = append(values, sql.Named("status", "{}")) // status
	}
	values = append(values, sql.Named("subscriptionlist_id", feed.SubscriptionListID)) // subscriptionlist_id
	if len(feed.Website.HTMLURL) > 0 {
		values = append(values, sql.Named("website_id", feed.Website.Identifier())) // website_id
	} else {
		values = append(values, sql.Named("website_id", nil)) // website_id
	}
	return values
}
func (feed *Feed) DatabaseUpdatesSet(_ []string) []string {
	return []string{
		"url = excluded.url",
		"kind = excluded.kind",
		"html_url = excluded.html_url",
		"title = excluded.title",
		"description = excluded.description",
		"language = excluded.language",
		"is_podcast = excluded.is_podcast",
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
