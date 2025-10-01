package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/herdworks/hyperfeed-go/state"
	"github.com/herdworks/hyperfeed-go/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// errors
var (
	ErrNotImplemented   = errors.New("not implemented")
	ErrSchedulerRequest = errors.New("invalid scheduler request")
)

// defaults
var (
	DefaultSchedulerConfig = &Config{
		DefaultInterval:    15 * time.Minute,
		MinInterval:        time.Minute,
		MaxInterval:        12 * time.Hour,
		StartupGracePeriod: 5 * time.Minute,
	}
)

// Scheduler manages feed updates using persistent state and the transporter.
type Scheduler struct {
	Feed   *hyperfeed.Feed
	Config *Config
	Status *Status
	Clock  Clock

	State       *state.State
	Transporter *transporter.Transporter
}

// Config provides runtime configuration for a Scheduler instance.
// TODO: should this become a Scheduler.Interval field and a SubscriberConfig struct?
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

// Status captures everything the scheduler must persist across ticks.
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
// The scheduler persists this structure across ticks.
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

// ========================= Scheduler Methods =============================

// Tick computes the scheduler's next event. Implementation must load
// state, honor rate-limit hints, and update persistence optimistically to avoid
// duplicate work after crashes.
func (s *Scheduler) Tick(ctx context.Context) error {
	var request *transporter.Request
	var response *transporter.Response
	var err error
	if ready, reason := s.Ready(); !ready {
		return fmt.Errorf("not ready: %s", reason)
	}
	if request, err = s.Request(); err != nil {
		return err
	}
	if s.Transporter == nil {
		s.Transporter = &transporter.Transporter{}
	}
	if response, err = s.Transporter.Fetch(ctx, request); err != nil {
		return s.Log(response, err)
	}
	return s.Apply(ctx, response)
}

// Ready determines if the Scheduler is ready to be fetched again.
func (s *Scheduler) Ready() (ready bool, reason string) {
	var tick time.Time = time.Now().UTC()
	if s.Status == nil {
		return true, "new feed"
	}
	if tick.Before(s.Status.UpdatedAt.Add(s.Config.MinInterval)) {
		return false, "updated too recently"
	}
	if tick.Before(s.Status.PausedUntil) {
		return false, fmt.Sprintf("paused until %s", s.Status.PausedUntil)
	}
	if tick.Before(s.Status.RateLimit.WaitUntil) {
		return false, fmt.Sprintf("rate limited until %s", s.Status.RateLimit.WaitUntil)
	}
	if retryAfter := s.Status.LastAttemptAt.Add(s.Status.RateLimit.RetryAfter); tick.Before(retryAfter) {
		return false, fmt.Sprintf("rate limited until %s", retryAfter)
	}
	return true, "ready"
}

// Request builds a transporter.Request for the current feed and status.
func (s *Scheduler) Request() (request *transporter.Request, err error) {
	if s.Feed == nil || s.Feed.URL == nil {
		return nil, errors.Join(ErrSchedulerRequest, fmt.Errorf("invalid feed"))
	}
	request = &transporter.Request{
		URL:        s.Feed.URL,
		Conditions: s.Conditions(),
	}
	return request, nil
}

// Conditions generates a list of request conditions based on the current event state.
func (scheduler *Scheduler) Conditions() (conditions []transporter.Condition) {
	if scheduler.Status == nil {
		return nil
	}
	// Default conditions
	conditions = append(
		conditions,
		transporter.Condition{
			Kind:      transporter.ConditionLastOK,
			TimeValue: scheduler.Status.LastSuccessAt,
		},
		transporter.Condition{
			Kind:          transporter.ConditionMinInterval,
			DurationValue: scheduler.Config.MinInterval,
		},
		transporter.Condition{
			Kind:          transporter.ConditionMaxInterval,
			DurationValue: scheduler.Config.MaxInterval,
		},
		transporter.Condition{
			Kind:          transporter.ConditionRateLimit,
			TimeValue:     scheduler.Status.RateLimit.WaitUntil,
			DurationValue: scheduler.Status.RateLimit.RetryAfter,
		},
	)

	// ETag validator
	if scheduler.Status.ETag != "" {
		conditions = append(conditions, transporter.Condition{
			Kind:  transporter.ConditionIfNoneMatch,
			Value: scheduler.Status.ETag,
		})
	}

	// Last-Modified validator
	if !scheduler.Status.LastModified.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfModifiedSince,
			TimeValue: scheduler.Status.LastModified,
		})
	}

	// Rate limit guardrails
	if !scheduler.Status.RateLimit.WaitUntil.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfUnmodifiedSince,
			Name:      "X-RateLimit-Wait-Until",
			TimeValue: scheduler.Status.RateLimit.WaitUntil,
		})
	}
	return conditions
}

// Apply updates the scheduler state based on the transporter response.
// Implementation must update error counters, compute backoff, and persist
// state.
func (s *Scheduler) Apply(ctx context.Context, response *transporter.Response) error {
	if s.Status == nil {
		s.Status = &Status{LastStatusCodes: []int{}}
	}
	if response == nil || response.Request == nil {
		s.Status.LastAttemptAt = time.Now().UTC()
	}
	s.ApplyTimestamps(response)
	s.ApplyStatusCode(response)
	s.ApplyHeaders(response)
	s.ApplyRateLimits(response)
	s.ApplyBody(response)
	return nil
}
func (s *Scheduler) ApplyTimestamps(response *transporter.Response) {
	if s.Status.CreatedAt.IsZero() {
		s.Status.CreatedAt = response.ReceivedAt.UTC()
	}
	s.Status.LastAttemptAt = response.ReceivedAt.UTC()
	if response.OK() {
		s.Status.UpdatedAt = response.ReceivedAt.UTC()
		s.Status.LastSuccessAt = response.ReceivedAt.UTC()
	} else {
		s.Status.LastErrorAt = response.ReceivedAt.UTC()
	}
}
func (s *Scheduler) ApplyStatusCode(response *transporter.Response) {
	var limit int = min(29, len(s.Status.LastStatusCodes))
	s.Status.LastStatusCode = response.StatusCode
	s.Status.LastStatusCodes = append([]int{response.StatusCode}, s.Status.LastStatusCodes[:limit]...)
}
func (s *Scheduler) ApplyHeaders(response *transporter.Response) {
	s.Status.Headers = response.Response.Header.Clone()
	if etag := response.Response.Header.Get("ETag"); etag != "" {
		s.Status.ETag = etag
	}
	if response.Response.ContentLength > 0 {
		s.Status.ContentLength = response.Response.ContentLength
	}
	s.Status.LastModified = response.LastModified()
}
func (s *Scheduler) ApplyRateLimits(response *transporter.Response) {
	s.Status.RateLimit.WaitUntil = response.WaitUntil()
	s.Status.RateLimit.RetryAfter = response.RetryAfter()
	s.Status.RateLimit.PollInterval = response.PollInterval()
	s.Status.RateLimit.Recent429s = count(s.Status.LastStatusCodes, func(code int) bool { return code == 429 })
	s.Status.RateLimit.Recent5xxs = count(s.Status.LastStatusCodes, func(code int) bool { return code >= 500 })
}
func (s *Scheduler) ApplyBody(response *transporter.Response) {
	s.Feed.Bytes = response.Bytes
	// TODO: parse feed body and update s.Status.FeedTTL or s.Status.RateLimit.WaitUntil?
	// TODO: parse feed body and update FeedKind?
}

// Log persists failure information for the current tick.
func (s *Scheduler) Log(response *transporter.Response, err error) error {
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
