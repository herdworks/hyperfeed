package hyperfeed

import (
	"net/http"
	"time"
)

// TYPES
// =============================================================================

// Status represents fetch state (schedule, rate limiting, etc)
type Status struct {
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

	// LastEntityUpdate
	LastEntityUpdateAt time.Time `json:"last_entity_update_at"`

	// LastEntityError
	LastEntityError string `json:"last_entity_error"`

	// LastEntityHeaders
	LastEntityHeaders http.Header `json:"last_entity_headers"`

	// RateLimit captures rate limiting information supplied by the origin.
	RateLimit FeedRateLimit `json:"rate_limit"`

	// CacheControl stores parsed HTTP cache headers.
	CacheControl FeedCacheControl `json:"cache_control"`

	// Headers stores the raw headers from the most recent fetch to support future hint extraction.
	Headers http.Header `json:"headers"`
}

// METHODS
// =============================================================================

func (status *Status) Cooldown(interval time.Duration) (cooldown time.Duration) {
	if status == nil {
		return 0
	}
	var intervalUnits time.Duration
	var notmodifieds int = count(status.LastStatusCodes, 304)
	var requesterrs int = count(status.LastStatusCodes, 400, 401, 403, 404, 410)
	var servererrs int = count(status.LastStatusCodes, 500, 502, 503, 504, 520)
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
			status.FetchErrors, // also penalize consecutive request errors
	) * intervalUnits
}

func (status *Status) Fetched() bool {
	return status.UpdatedAt == status.LastSuccessAt
}

func (status *Status) Skipped() bool {
	return status.UpdatedAt != status.LastAttemptAt
}

func (status *Status) Failed() bool {
	switch {
	case status.UpdatedAt == status.LastAttemptAt && status.FetchErrors > 0:
		return true
	case status.UpdatedAt == status.LastAttemptAt && status.UpdatedAt == status.LastErrorAt:
		return true
	default:
		return false
	}
}
