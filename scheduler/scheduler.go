package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/herdworks/hyperfeed-go/state"
	"github.com/herdworks/hyperfeed-go/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// errors
var (
	ErrNotImplemented   = errors.New("scheduler: not implemented")
	ErrSchedulerRequest = errors.New("scheduler: invalid request")
)

// defaults
var (
	DefaultSchedulerConfig = &Config{
		DefaultInterval:        15 * time.Minute,
		MinInterval:            time.Minute,
		MaxInterval:            12 * time.Hour,
		StartupGracePeriod:     5 * time.Minute,
		ErrorBackoffStep:       time.Minute,
		DisableFeedHints:       false,
		DisableCacheDirectives: false,
		DisableRateLimitHints:  false,
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
type Config struct {
	// DefaultInterval is the baseline polling interval when no hints are available.
	// Implementations must never schedule requests more frequently than this value.
	DefaultInterval time.Duration

	// MinInterval constrains how aggressively we can poll, even when feeds publish explicit TTL hints.
	// Implementations must clamp decisions to this floor to avoid overwhelming origins.
	MinInterval time.Duration

	// MaxInterval caps how stale a feed is allowed to become.
	// Implementations must clamp decisions to this ceiling even when the origin suggests very long TTLs, unless the operator explicitly overrides the value.
	MaxInterval time.Duration

	// StartupGracePeriod allows newly subscribed feeds to be fetched more aggressively during onboarding without violating server hints.
	StartupGracePeriod time.Duration

	// ErrorBackoffStep defines the base duration added per consecutive error.
	// Scheduler multiplies this by the failure count when no Retry-After hint is available.
	ErrorBackoffStep time.Duration

	// DisableFeedHints turns off enforcement of RSS feed cadence hints (skipDays, skipHours, ttl).
	DisableFeedHints bool

	// DisableCacheDirectives turns off enforcement of HTTP cache headers.
	DisableCacheDirectives bool

	// DisableRateLimitHints turns off enforcement of Retry-After and related rate limit guidance.
	DisableRateLimitHints bool
}

// Status captures everything the scheduler must persist across ticks.
// The contract intentionally mirrors HTTP semantics and feed hints so we can satisfy the requirements outlined in the Feed Reader Behavior document.
type Status struct {
	// FeedID is stored redundantly to guard against persistence mix-ups.
	FeedID string

	// CreatedAt indicates when the feed was first scheduled.
	CreatedAt time.Time

	// UpdatedAt indicates when the state was last modified.
	UpdatedAt time.Time

	// LastAttemptAt is when we most recently attempted a fetch, successful or not.
	LastAttemptAt time.Time

	// LastErrorAt is when we most recently encountered an HTTP 4xx or 5xx error.
	LastErrorAt time.Time

	// LastSuccessAt is when we most recently fetched new content.
	LastSuccessAt time.Time

	// LastStatusCode stores the HTTP status code returned on the last attempt, including 304 and error codes.
	LastStatusCode int

	// LastStatusCodes stores the last 30 status codes for diagnostic purposes.
	LastStatusCodes []int

	// LastHeaders stores the raw headers from the most recent fetch to support future hint extraction.
	LastHeaders http.Header

	// LastModifiedAt stores the value of the Last-Modified header from the last successful fetch.
	LastModifiedAt time.Time

	// LastContentLength is the size of the payload from the last successful fetch.
	LastContentLength int64

	// ETag is the last/current ETag value seen for this feed.
	ETag string

	// CacheControl stores parsed HTTP cache headers.
	CacheControl CacheControl

	// RateLimit captures rate limiting information supplied by the origin.
	RateLimit RateLimit

	// PausedUntil stores user-imposed pause windows (manual snooze).
	PausedUntil time.Time

	// SkipDays enumerates which weekdays should be skipped according to feed metadata.
	// SkipDays map[time.Weekday]bool

	// SkipHours enumerates which hours (0-23) should be skipped according to feed metadata.
	// SkipHours map[int]bool

	// NextNotBefore stores the earliest permissible fetch time derived from hints and policy. Scheduler must never schedule before this timestamp.
	// NextNotBefore time.Time

	// Diagnostics retains arbitrary key/value pairs for debugging decisions (e.g., "retry_after": "2024-12-17T15:04Z").
	// Diagnostics map[string]string

	// FeedTTL holds the last ttl hint observed from the feed metadata.
	// FeedTTL time.Duration
}

// RateLimit captures how often we are allowed to poll an origin as signaled by HTTP 429/503 and related headers.
// The scheduler persists this structure across ticks.
type RateLimit struct {
	WaitUntil             time.Time     // Self-imposed backoff time
	BackoffUntil          time.Time     // Retry-After or computed backoff time
	RetryAfter            time.Duration // Retry-After in seconds
	Remaining             int           // X-RateLimit-Remaining
	Reset                 time.Time     // X-RateLimit-Reset
	Window                string        // X-RateLimit-Window
	SuggestedPollInterval time.Duration // X-Poll-Interval in seconds
	LastStatus            int           // last HTTP status code affecting rate limit state
	Consecutive429        int           // count of sequential 429 responses
	Consecutive5xx        int           // count of sequential 5xx responses
}

// CacheControl normalizes cache metadata from HTTP headers.
type CacheControl struct {
	Age          time.Duration
	MaxAge       time.Duration
	SharedMaxAge time.Duration
	ExpiresAt    time.Time
	ETagMismatch bool
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
		return fmt.Errorf("failed to build request: %w", err)
	}
	if s.Transporter == nil {
		s.Transporter = &transporter.Transporter{}
	}
	if response, err = s.Transporter.Fetch(ctx, request); err != nil {
		return fmt.Errorf("fetch error: %w", err)
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
	if tick.Before(s.Status.RateLimit.BackoffUntil) {
		return false, fmt.Sprintf("backing off until %s", s.Status.RateLimit.BackoffUntil)
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
	)

	// Last-Modified validator
	if !scheduler.Status.LastModifiedAt.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfModifiedSince,
			TimeValue: scheduler.Status.LastModifiedAt,
		})
	}

	// ETag validator
	if scheduler.Status.ETag != "" {
		conditions = append(conditions, transporter.Condition{
			Kind:  transporter.ConditionIfNoneMatch,
			Value: scheduler.Status.ETag,
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

	// timestamps
	s.Status.LastAttemptAt = time.Now().UTC()
	if s.Status.CreatedAt.IsZero() {
		s.Status.CreatedAt = s.Status.LastAttemptAt
	}
	if response.OK() {
		s.Status.LastSuccessAt = s.Status.LastAttemptAt
	} else {
		s.Status.LastErrorAt = s.Status.LastAttemptAt
	}

	// status codes
	limit := min(29, len(s.Status.LastStatusCodes))
	s.Status.LastStatusCode = response.StatusCode
	s.Status.LastStatusCodes = append([]int{response.StatusCode}, s.Status.LastStatusCodes[:limit]...)
	if response.OK() {
		s.Status.UpdatedAt = s.Status.LastAttemptAt
		s.Status.LastSuccessAt = s.Status.LastAttemptAt
	}

	// headers
	s.Status.LastHeaders = response.Response.Header.Clone()
	if etag := response.Response.Header.Get("ETag"); etag != "" {
		s.Status.ETag = etag
	}
	if cl, err := strconv.Atoi(response.Response.Header.Get("Content-Length")); err == nil && cl >= 0 {
		s.Status.LastContentLength = int64(cl)
	}
	if lm := response.LastModified(); !lm.IsZero() {
		s.Status.LastModifiedAt = lm
	}

	return ErrNotImplemented
}

// Log persists failure information when the transporter fails before
// producing a response. Implementation must update error counters and
// compute a conservative backoff. Stub returns ErrNotImplemented.
func (s *Scheduler) Log(ctx context.Context, status *Status, request *transporter.Request, err error) (*Status, error) {
	return nil, ErrNotImplemented
}

// ========================= Config Methods ===================================

// Normalize ensures all Config fields have sane defaults and bounds.
func (cfg *Config) Normalize() {
	defaults := *DefaultSchedulerConfig // copy
	if cfg == nil {
		return
	}
	if cfg.DefaultInterval == 0 {
		cfg.DefaultInterval = defaults.DefaultInterval
	}
	if cfg.MinInterval == 0 {
		cfg.MinInterval = defaults.MinInterval
	}
	if cfg.MaxInterval == 0 {
		cfg.MaxInterval = defaults.MaxInterval
	}
	if cfg.StartupGracePeriod == 0 {
		cfg.StartupGracePeriod = defaults.StartupGracePeriod
	}
	if cfg.ErrorBackoffStep == 0 {
		cfg.ErrorBackoffStep = defaults.ErrorBackoffStep
	}
	if cfg.MinInterval > cfg.DefaultInterval {
		cfg.MinInterval = cfg.DefaultInterval
	}
	if cfg.MaxInterval < cfg.DefaultInterval {
		cfg.MaxInterval = cfg.DefaultInterval
	}
	if cfg.MinInterval > cfg.MaxInterval {
		cfg.MinInterval = cfg.MaxInterval
	}
}

// ========================= Status Methods ===================================
