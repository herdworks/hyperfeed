package hyperfeed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// ============================ Package Types =================================

// Feed represents a single RSS/Atom feed.
type Feed struct {
	ID          string      `json:"id,omitempty"`  // stable HyperFeed Feed identifier (normalized URL string)
	URL         *href.Href  `json:"url,omitempty"` // canonical feed URL (RSS/Atom endpoint)
	HTMLURL     *href.Href  `json:"html_url,omitempty"`
	UpdatedAt   time.Time   `json:"updated_at,omitempty"`
	Language    string      `json:"language,omitempty"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	Authors     []Author    `json:"authors,omitempty"`
	Metadata    Metadata    `json:"metadata,omitempty"` // Non-standard/extension properties
	Items       []*Item     `json:"items,omitempty"`    // the most recent items for this feed
	Status      *FeedStatus `json:"status,omitempty"`   // fetch/status metadata

	Bytes []byte `json:"-"` // the most recent contents (raw bytes) for this feed
}

// FeedStatus represents feed state (schedule, rate limiting, etc)
type FeedStatus struct {
	// CreatedAt indicates when the feed was first scheduled.
	// CreatedAt is only set once, when the feed is first added to the database.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt indicates when the state was last updated.
	// UpdatedAt is set on every tick.
	UpdatedAt time.Time `json:"updated_at"`

	// LastAttemptAt is when we most recently attempted a fetch, successful or not.
	// LastAttemptAt is only set on ticks when a fetch is attempted.
	LastAttemptAt time.Time `json:"last_attempt_at"`

	// LastSuccessAt is when we most recently fetched new content.
	// LastSuccessAt is only set on fetches that return 2xx or 304 Not Modified status codes.
	LastSuccessAt time.Time `json:"last_success_at"`

	// LastErrorAt is when we most recently encountered an HTTP 4xx or 5xx error.
	// LastErrorAt is only set on fetches that return 4xx or 5xx status codes.
	LastErrorAt time.Time `json:"last_error_at"`

	// LastError stores the last error message encountered during fetch attempts.
	// LastError is set on every fetch; if the fetch is successful, LastError is cleared.
	LastError string `json:"last_error"`

	// FetchErrors counts consecutive request errors (DNS, TLS, network, etc)
	// FetchErrors is incremented on every request error, and reset to zero on a successful fetch.
	FetchErrors int `json:"fetch_errors"`

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

// ============================ Package Variables =============================

// ============================ Feed Methods ==================================

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

func (feed *Feed) GetURL() *url.URL {
	if feed == nil {
		return nil
	}
	if feed.HTMLURL != nil && feed.HTMLURL.URL().IsAbs() {
		return feed.HTMLURL.URL()
	}
	if feed.URL != nil && feed.URL.URL().IsAbs() {
		return feed.URL.URL()
	}
	return nil
}

// DatabaseReadable interface methods
func (Feed) DatabaseView() string {
	return "feeds_view"
}
func (Feed) DatabaseViewIdentifier() string {
	return "id"
}
func (Feed) DatabaseObject() string {
	return "feed"
}

// DatabaseWritable interface methods
func (Feed) DatabaseTable() string {
	return "feeds"
}
func (Feed) DatabaseTableIdentifier() string {
	return "id"
}
func (Feed) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"url",
		"html_url",
		"updated_at",
		"title",
		"description",
		"language",
		"status",
	}
}
func (feed *Feed) DatabaseInsertValues() (values []any) {
	statusBytes, _ := json.Marshal(feed.Status)
	values = append(values, feed.Identifier())                           // id
	values = append(values, normalized.URL(feed.URL.URL()).String())     // url
	values = append(values, normalized.URL(feed.HTMLURL.URL()).String()) // html_url
	values = append(values, feed.UpdatedAt.UTC().Format(time.RFC3339))   // updated_at
	values = append(values, feed.Title)                                  // title
	values = append(values, feed.Description)                            // description
	values = append(values, feed.Language)                               // language
	values = append(values, string(statusBytes))                         // status
	return values
}
func (feed *Feed) DatabaseUpdatesSet() []string {
	return []string{
		"url = excluded.url",
		"html_url = excluded.html_url",
		"updated_at = excluded.updated_at",
		"title = excluded.title",
		"description = excluded.description",
		"language = excluded.language",
		"status = excluded.status",
	}
}
func (Feed) DatabaseWrite() string {
	columns := []string{}
	columns = append(columns, "id")
	columns = append(columns, "url")
	columns = append(columns, "html_url")
	columns = append(columns, "updated_at")
	columns = append(columns, "title")
	columns = append(columns, "description")
	columns = append(columns, "language")
	columns = append(columns, "status")
	bindings := []string{}
	for range columns {
		bindings = append(bindings, "?")
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) ", Feed{}.DatabaseTable(), strings.Join(columns, ", "))
	query += fmt.Sprintf("VALUES (%s) ", strings.Join(bindings, ", "))
	return query
}
func (Feed) DatabaseWriteOrReplace() string {
	var query string = Feed{}.DatabaseWrite()
	var columns []string = []string{}
	query += fmt.Sprintf("ON CONFLICT(%s) DO UPDATE SET ", Feed{}.DatabaseTableIdentifier())
	columns = append(columns, "url = excluded.url")
	columns = append(columns, "html_url = excluded.html_url")
	columns = append(columns, "updated_at = excluded.updated_at")
	columns = append(columns, "title = excluded.title")
	columns = append(columns, "description = excluded.description")
	columns = append(columns, "language = excluded.language")
	columns = append(columns, "status = excluded.status")
	query += " " + strings.Join(columns, ", ")
	return query
}

// Hash computes a simple hash of the feed's significant fields for change detection.
// TODO: is this needed for anything other than testing?
func (feed *Feed) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s",
		feed.Identifier(),
		normalized.URL(feed.URL.URL()).String(),
		normalized.URL(feed.HTMLURL.URL()).String(),
		feed.UpdatedAt.UTC().Format(time.RFC3339),
		feed.Title,
		feed.Description,
		feed.Language,
	)
}

// ============================ Status Methods ================================

// TBD if this is helpful, but it feels like an interesting experiment to optimize refresh workloads.
func (status *FeedStatus) Cooldown(interval time.Duration) (cooldown time.Duration) {
	if status == nil {
		return 0
	}
	var intervalUnits time.Duration
	var notmodifieds int = count(status.LastStatusCodes, 304)
	var requesterrs int = count(status.LastStatusCodes, 400, 401, 403, 404, 410)
	var servererrs int = count(status.LastStatusCodes, 500, 502, 503, 504, 520)
	var toomanyreqs int = count(status.LastStatusCodes, 429)
	switch {
	case interval < 15*time.Minute:
		intervalUnits = 30 * time.Second // +/- 0-15 minutes
	case interval < 30*time.Minute:
		intervalUnits = 1 * time.Minute // +/- 0-30 minutes
	case interval < 60*time.Minute:
		intervalUnits = 6 * time.Minute // +/- 0-180 minutes (0-3 hours)
	case interval < 240*time.Minute:
		intervalUnits = 40 * time.Minute // +/- 0-1200 minutes (0-20 hours)
	case interval >= 1440*time.Minute:
		intervalUnits = 0 * time.Minute // 24 hours is the max
	}
	return time.Duration(
		notmodifieds+
			requesterrs+
			servererrs+
			toomanyreqs+
			status.FetchErrors, // also penalize consecutive request errors
	) * intervalUnits
}

func (status *FeedStatus) Fetched() bool {
	return status.UpdatedAt == status.LastSuccessAt
}

func (status *FeedStatus) Skipped() bool {
	return status.UpdatedAt != status.LastAttemptAt
}

func (status *FeedStatus) Failed() bool {
	switch {
	case status.UpdatedAt == status.LastAttemptAt && status.FetchErrors > 0:
		return true
	case status.UpdatedAt == status.LastAttemptAt && status.UpdatedAt == status.LastErrorAt:
		return true
	default:
		return false
	}
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
