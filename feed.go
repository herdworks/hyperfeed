package hyperfeed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"time"

	"github.com/herdworks/hq"
	"github.com/herdworks/hyperfeed/href"
	normalized "github.com/herdworks/hyperfeed/internal/normalization"
)

// TYPES
// =============================================================================

// Feed represents a single RSS/Atom feed.
type Feed struct {
	*DatabaseRecord

	ID          string        `json:"id,omitempty"`       // stable HyperFeed Feed identifier (normalized URL string)
	Kind        string        `json:"kind"`               // feed kind (e.g. "rss", "atom", "jsonfeed", "invalid", "unknown", etc.)
	URL         *href.Href    `json:"url,omitempty"`      // canonical feed URL (RSS/Atom endpoint)
	HTMLURL     *href.Href    `json:"html_url,omitempty"` // human-readable feed URL (web page)
	Email       string        `json:"email,omitempty"`    // contact email address for the feed (if available)
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	Language    string        `json:"language,omitempty"`
	IsPodcast   bool          `json:"is_podcast,omitempty"`
	Following   bool          `json:"following"`
	Metadata    Metadata      `json:"metadata,omitempty"` // Non-standard/extension properties
	Interval    time.Duration `json:"interval,omitempty"` // fetch interval in minutes
	UpdatedAt   time.Time     `json:"updated_at,omitempty"`
	Website     Website       `json:"website,omitempty"` // the Website this feed is associated with
	Status      Status        `json:"status,omitempty"`  // fetch status

	// Publishing metadata is carried in memory and JSON, not database rows.
	Bylines    []Byline  `json:"bylines,omitempty"`
	Links      []*Link   `json:"links,omitempty"`
	ModifiedAt time.Time `json:"modified_at,omitempty"` // source feed modification time

	Items []*Item `json:"items,omitempty"` // the most recent items for this feed
	Bytes []byte  `json:"-"`               // the most recent contents (raw bytes) for this feed
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

// MarshalJSON omits an unspecified source modification time.
func (feed Feed) MarshalJSON() ([]byte, error) {
	type feedJSON Feed
	var modifiedAt *time.Time
	if !feed.ModifiedAt.IsZero() {
		modifiedAt = &feed.ModifiedAt
	}
	return json.Marshal(struct {
		feedJSON
		ModifiedAt *time.Time `json:"modified_at,omitempty"`
	}{feedJSON: feedJSON(feed), ModifiedAt: modifiedAt})
}

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

// WebsiteURL returns the feed's website URL root
func (feed *Feed) WebsiteURL() *url.URL {
	if feed == nil || feed.HTMLURL == nil || feed.HTMLURL.String() == "" {
		return nil
	}
	website := *feed.HTMLURL.URL()
	website.Path = "/"
	website.RawQuery = ""
	website.ForceQuery = false
	website.Fragment = ""
	return &website
}

// WebsiteIdentifier returns a stable website identifier derived from Feed.HTMLURL.
func (feed *Feed) WebsiteIdentifier() string {
	if feed == nil || feed.HTMLURL == nil || feed.HTMLURL.String() == "" {
		return normalized.Identifier("")
	}
	if website := feed.WebsiteURL(); website == nil {
		return normalized.Identifier("")
	} else {
		return normalized.Identifier(normalized.URL(website).String())
	}
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
	if feed.HTMLURL == nil {
		return nil // nothing to scrape
	}
	if scheme := feed.HTMLURL.URL().Scheme; scheme != "http" && scheme != "https" {
		return nil // local/non-web feeds have nothing to scrape
	}
	var document *hq.Document = &hq.Document{Href: feed.HTMLURL.String()}
	if w == nil {
		w = os.Stdout
	}
	if !feed.Status.LastEntityUpdateAt.IsZero() && time.Since(feed.Status.LastEntityUpdateAt) < 24*time.Hour {
		return nil // recently updated
	}
	if err = document.Download(ctx, true, nil, int(timeout/1000), w); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("downloading document"), err)
	}
	if err = document.Scrape(ctx, true, nil, int(timeout/1000), w); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("scraping document"), err)
	}
	feed.Website.Entity = document.Metadata.Entity
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
func (*Feed) DatabaseView() string {
	return "feeds_view"
}
func (*Feed) DatabaseTable() string {
	return "feeds"
}
func (*Feed) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"kind",
		"url",
		"html_url",
		"email",
		"title",
		"description",
		"language",
		"is_podcast",
		"interval",
		"following",
		"status",
		"website_id",
	}
}
func (feed *Feed) DatabaseInsertValues() (values []any) {
	values = append(values, sql.Named("id", feed.Identifier()))
	values = append(values, sql.Named("kind", feed.Kind))
	values = append(values, sql.Named("url", feed.URL.URL().String()))
	if feed.HTMLURL != nil {
		values = append(values, sql.Named("html_url", feed.HTMLURL.URL().String()))
	} else {
		values = append(values, sql.Named("html_url", nil))
	}
	values = append(values, sql.Named("email", feed.Email))
	values = append(values, sql.Named("title", feed.Title))
	values = append(values, sql.Named("description", feed.Description))
	values = append(values, sql.Named("language", feed.Language))
	values = append(values, sql.Named("is_podcast", feed.IsPodcast))
	values = append(values, sql.Named("interval", feed.Interval.String()))
	values = append(values, sql.Named("following", feed.Following))
	if statusBytes, _ := json.Marshal(feed.Status); statusBytes != nil {
		values = append(values, sql.Named("status", string(statusBytes)))
	} else {
		values = append(values, sql.Named("status", "{}"))
	}
	if len(feed.Website.HTMLURL) > 0 {
		values = append(values, sql.Named("website_id", feed.Website.Identifier()))
	} else {
		values = append(values, sql.Named("website_id", nil))
	}
	return values
}
func (feed *Feed) DatabaseUpdatesSet(_ []string) []string {
	return []string{
		"url = excluded.url",
		"kind = excluded.kind",
		"html_url = excluded.html_url",
		"email = excluded.email",
		"title = excluded.title",
		"description = excluded.description",
		"language = excluded.language",
		"is_podcast = excluded.is_podcast",
		"interval = excluded.interval",
		"following = excluded.following",
		"status = excluded.status",
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
