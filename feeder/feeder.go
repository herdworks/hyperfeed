package feeder

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"

	"github.com/herdworks/hyperfeed-go/internal/state"
	"github.com/herdworks/hyperfeed-go/internal/transporter"
)

// ============================ Package Types =================================

// Feeder manages a Feed.
type Feeder struct {
	*hyperfeed.Feed

	Config *Config
	Status *Status
	Clock  Clock

	State       *state.State
	Transporter *transporter.Transporter
}

// Config provides runtime configuration for a Feeder instance.
// TODO: should this become a Feeder.Interval field and a SubscriberConfig struct?
type Config struct {
	// DefaultInterval is the baseline polling interval when no hints are available.
	// Implementations must never schedule requests more frequently than this value.
	DefaultInterval time.Duration `json:"default_interval"`

	// MinInterval constrains how aggressively we can poll, even when feeds publish explicit TTL hints.
	// Implementations must clamp decisions to this floor to avoid overwhelming origins.
	MinInterval time.Duration `json:"min_interval"`

	// MaxInterval caps how stale a feed is allowed to become.
	// Implementations must clamp decisions to this ceiling even when the origin suggests very long TTLs, unless the operator explicitly overrides the value.
	MaxInterval time.Duration `json:"max_interval"`

	// StartupGracePeriod allows newly subscribed feeds to be fetched more aggressively during onboarding without violating server hints.
	StartupGracePeriod time.Duration `json:"startup_grace_period"`
}

// Status captures everything the feeder must persist across ticks.
// The contract intentionally mirrors HTTP semantics and feed hints so we can satisfy the requirements outlined in the Feed Reader Behavior document.
type Status struct {
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

	// CacheControl stores parsed HTTP cache headers.
	CacheControl CacheControl `json:"cache_control"`

	// RateLimit captures rate limiting information supplied by the origin.
	RateLimit RateLimit `json:"rate_limit"`

	// Headers stores the raw headers from the most recent fetch to support future hint extraction.
	Headers http.Header `json:"headers"`
}

// RateLimit captures how often we are allowed to poll an origin as signaled by HTTP 429/503 and related headers.
// The feeder persists this structure across ticks.
type RateLimit struct {
	WaitUntil    time.Time     `json:"wait_until"`    // Retry-After time
	RetryAfter   time.Duration `json:"retry_after"`   // Retry-After seconds
	PollInterval time.Duration `json:"poll_interval"` // X-Poll-Interval in seconds
	Recent429s   int           `json:"recent_429s"`   // count of 429 responses in the last 30 requests
	Recent5xxs   int           `json:"recent_5xxs"`   // count of 5xx responses in the last 30 requests
	// Remaining             int           `json:"remaining"` // X-RateLimit-Remaining
	// Reset                 time.Time     `json:"reset"` // X-RateLimit-Reset
	// Window                string        `json:"window"` // X-RateLimit-Window
}

// CacheControl normalizes cache metadata from HTTP headers.
type CacheControl struct {
	Age          time.Duration `json:"age"`
	MaxAge       time.Duration `json:"max_age"`
	SharedMaxAge time.Duration `json:"shared_max_age"`
	ExpiresAt    time.Time     `json:"expires_at"`
	ETagMismatch bool          `json:"etag_mismatch"`
}

// Clock abstracts time so tests can inject deterministic clocks.
type Clock interface {
	Now() time.Time
}

// ============================ Package Variables =============================

// errors
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrRequest        = errors.New("request error")
)

// defaults
var (
	DefaultFeederConfig = &Config{
		DefaultInterval:    15 * time.Minute,
		MinInterval:        time.Minute,
		MaxInterval:        12 * time.Hour,
		StartupGracePeriod: 5 * time.Minute,
	}
)

// ========================= Feeder Methods =============================

// Tick computes the feeder's next event. Implementation must load
// state, honor rate-limit hints, and update persistence optimistically to avoid
// duplicate work after crashes.
func (feeder *Feeder) Tick(ctx context.Context) error {
	var request *transporter.Request
	var response *transporter.Response
	var err error
	if ready, reason := feeder.Ready(); !ready {
		return fmt.Errorf("not ready: %s", reason)
	}
	if request, err = feeder.Request(); err != nil {
		return err
	}
	if feeder.Transporter == nil {
		feeder.Transporter = &transporter.Transporter{}
	}
	if response, err = feeder.Transporter.Fetch(ctx, request); err != nil {
		return feeder.Log(response, err)
	}
	return feeder.Apply(ctx, response)
}

// Ready determines if the Feeder is ready to be fetched again.
func (feeder *Feeder) Ready() (ready bool, reason string) {
	var tick time.Time = time.Now().UTC()
	if feeder.Status == nil {
		return true, "new feed"
	}
	if tick.Before(feeder.Status.UpdatedAt.Add(feeder.Config.MinInterval)) {
		return false, "updated too recently"
	}
	if tick.Before(feeder.Status.PausedUntil) {
		return false, fmt.Sprintf("paused until %s", feeder.Status.PausedUntil)
	}
	if tick.Before(feeder.Status.RateLimit.WaitUntil) {
		return false, fmt.Sprintf("rate limited until %s", feeder.Status.RateLimit.WaitUntil)
	}
	if retryAfter := feeder.Status.LastAttemptAt.Add(feeder.Status.RateLimit.RetryAfter); tick.Before(retryAfter) {
		return false, fmt.Sprintf("rate limited until %s", retryAfter)
	}
	return true, "ready"
}

// Request builds a transporter.Request for the current feed and status.
func (feeder *Feeder) Request() (request *transporter.Request, err error) {
	if feeder.Feed == nil || feeder.Feed.URL == nil {
		return nil, errors.Join(ErrRequest, fmt.Errorf("invalid feed"))
	}
	request = &transporter.Request{
		URL:        feeder.Feed.URL,
		Conditions: feeder.Conditions(),
	}
	return request, nil
}

// Conditions generates a list of request conditions based on the current event state.
func (feeder *Feeder) Conditions() (conditions []transporter.Condition) {
	if feeder.Status == nil {
		return nil
	}
	// Default conditions
	conditions = append(
		conditions,
		transporter.Condition{
			Kind:      transporter.ConditionLastOK,
			TimeValue: feeder.Status.LastSuccessAt,
		},
		transporter.Condition{
			Kind:          transporter.ConditionMinInterval,
			DurationValue: feeder.Config.MinInterval,
		},
		transporter.Condition{
			Kind:          transporter.ConditionMaxInterval,
			DurationValue: feeder.Config.MaxInterval,
		},
		transporter.Condition{
			Kind:          transporter.ConditionRateLimit,
			TimeValue:     feeder.Status.RateLimit.WaitUntil,
			DurationValue: feeder.Status.RateLimit.RetryAfter,
		},
	)

	// ETag validator
	if feeder.Status.ETag != "" {
		conditions = append(conditions, transporter.Condition{
			Kind:  transporter.ConditionIfNoneMatch,
			Value: feeder.Status.ETag,
		})
	}

	// Last-Modified validator
	if !feeder.Status.LastModified.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfModifiedSince,
			TimeValue: feeder.Status.LastModified,
		})
	}

	// Rate limit guardrails
	if !feeder.Status.RateLimit.WaitUntil.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfUnmodifiedSince,
			Name:      "X-RateLimit-Wait-Until",
			TimeValue: feeder.Status.RateLimit.WaitUntil,
		})
	}
	return conditions
}

// Apply updates the feeder state based on the transporter response.
// Implementation must update error counters, compute backoff, and persist
// state.
func (feeder *Feeder) Apply(ctx context.Context, response *transporter.Response) error {
	if feeder.Status == nil {
		feeder.Status = &Status{LastStatusCodes: []int{}}
	}
	if response == nil || response.Request == nil {
		feeder.Status.LastAttemptAt = time.Now().UTC()
	}
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.ApplyHeaders(response)
	feeder.ApplyRateLimits(response)
	feeder.ApplyBody(response)
	return nil
}
func (feeder *Feeder) ApplyTimestamps(response *transporter.Response) {
	if feeder.Status.CreatedAt.IsZero() {
		feeder.Status.CreatedAt = response.ReceivedAt.UTC()
	}
	feeder.Status.LastAttemptAt = response.ReceivedAt.UTC()
	if response.OK() {
		feeder.Status.UpdatedAt = response.ReceivedAt.UTC()
		feeder.Status.LastSuccessAt = response.ReceivedAt.UTC()
	} else {
		feeder.Status.LastErrorAt = response.ReceivedAt.UTC()
	}
}
func (feeder *Feeder) ApplyStatusCode(response *transporter.Response) {
	var limit int = min(29, len(feeder.Status.LastStatusCodes))
	feeder.Status.LastStatusCode = response.StatusCode
	feeder.Status.LastStatusCodes = append([]int{response.StatusCode}, feeder.Status.LastStatusCodes[:limit]...)
}
func (feeder *Feeder) ApplyHeaders(response *transporter.Response) {
	feeder.Status.Headers = response.Response.Header.Clone()
	if etag := response.Response.Header.Get("ETag"); etag != "" {
		feeder.Status.ETag = etag
	}
	if response.Response.ContentLength > 0 {
		feeder.Status.ContentLength = response.Response.ContentLength
	}
	feeder.Status.LastModified = response.LastModified()
}
func (feeder *Feeder) ApplyRateLimits(response *transporter.Response) {
	feeder.Status.RateLimit.WaitUntil = response.WaitUntil()
	feeder.Status.RateLimit.RetryAfter = response.RetryAfter()
	feeder.Status.RateLimit.PollInterval = response.PollInterval()
	feeder.Status.RateLimit.Recent429s = count(feeder.Status.LastStatusCodes, func(code int) bool { return code == 429 })
	feeder.Status.RateLimit.Recent5xxs = count(feeder.Status.LastStatusCodes, func(code int) bool { return code >= 500 })
}
func (feeder *Feeder) ApplyBody(response *transporter.Response) {
	feeder.Feed.Bytes = response.Bytes
	// TODO: parse feed body and update feeder.Status.FeedTTL or feeder.Status.RateLimit.WaitUntil?
	// TODO: parse feed body and update FeedKind?
}

// Log persists failure information for the current tick.
func (feeder *Feeder) Log(response *transporter.Response, err error) error {
	return ErrNotImplemented
}

// ========================= Helper Functions ===========================
func count[T any](slice []T, f func(T) bool) (count int) {
	for _, s := range slice {
		if f(s) {
			count++
		}
	}
	return count
}
