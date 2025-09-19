package scheduler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/herdworks/hyperfeed-go/state"
	"github.com/herdworks/hyperfeed-go/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

var (
	ErrNotImplemented = errors.New("scheduler: not implemented")
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

	// LastSuccessAt is when we most recently fetched new content.
	LastSuccessAt time.Time

	// LastStatusCode stores the HTTP status code returned on the last attempt, including 304 and error codes.
	LastStatusCode int

	// LastHeaders stores the raw headers from the most recent fetch to support future hint extraction.
	LastHeaders http.Header

	// LastError stores a diagnostic summary for the most recent failure.
	LastError string

	// ConsecutiveErrors counts sequential failed attempts.
	ConsecutiveErrors int

	// Conditions captures the validators for conditional requests.
	Conditions []*transporter.Condition

	// RateLimit captures rate limiting information supplied by the origin.
	RateLimit RateLimitState

	// CacheDirectives stores parsed HTTP cache headers.
	CacheDirectives CacheDirectiveState

	// FeedTTL holds the last ttl hint observed from the feed metadata.
	FeedTTL time.Duration

	// SkipDays enumerates which weekdays should be skipped according to feed metadata.
	SkipDays map[time.Weekday]bool

	// SkipHours enumerates which hours (0-23) should be skipped according to feed metadata.
	SkipHours map[int]bool

	// NextNotBefore stores the earliest permissible fetch time derived from hints and policy. Scheduler must never schedule before this timestamp.
	NextNotBefore time.Time

	// UserThrottleUntil stores user-imposed pause windows (manual snooze).
	UserThrottleUntil time.Time

	// Diagnostics retains arbitrary key/value pairs for debugging decisions (e.g., "retry_after": "2024-12-17T15:04Z").
	Diagnostics map[string]string
}

// RateLimitState captures how often we are allowed to poll an origin. The
// scheduler persists this structure across ticks.
type RateLimitState struct {
	BackoffUntil             time.Time
	RetryAfter               time.Duration
	Remaining                int
	Reset                    time.Time
	Window                   string
	Consecutive429           int
	Consecutive5xx           int
	SuggestedPollingInterval time.Duration
	LastStatus               int
	HintSources              []string
}

// CacheDirectiveState normalizes cache metadata from HTTP headers.
type CacheDirectiveState struct {
	MaxAge       time.Duration
	SharedMaxAge time.Duration
	ExpiresAt    time.Time
	Age          time.Duration
	ETagMismatch bool
}

// Clock abstracts time so tests can inject deterministic clocks.
type Clock interface {
	Now() time.Time
}

// Event captures the scheduler's evaluation snapshot for a single tick.
// It combines persisted state, feed configuration, and clock information so the transporter can be engaged without reloading broader context.
type Event struct {
	// Feed contains the immutable feed metadata used for this evaluation.
	Feed *hyperfeed.Feed

	// Status references the live scheduler status at the start of the tick.
	Status *Status

	// TickAt records when this event was generated.
	TickAt time.Time

	// SnapshotVersion identifies the logical version of the rate-limit and validator snapshot used while evaluating readiness.
	SnapshotVersion string

	// Diagnostics captures optional breadcrumbs explaining the evaluation.
	Diagnostics map[string]string
}

// ========================= Scheduler Methods =============================

// Tick computes the scheduler's next event. Implementation must load
// state, honor rate-limit hints, and update persistence optimistically to avoid
// duplicate work after crashes. Stub returns ErrNotImplemented.
func (s *Scheduler) Tick(ctx context.Context, feed *hyperfeed.Feed) (*Event, *Status, error) {
	return nil, nil, ErrNotImplemented
}

// Update ingests a transporter response and persists updated state. The
// implementation must combine HTTP hints, feed metadata, and error handling to
// compute the next eligible fetch time. Stub returns ErrNotImplemented.
func (s *Scheduler) Update(ctx context.Context, status *Status, request *transporter.Request, response *transporter.Response) (*Status, error) {
	return nil, ErrNotImplemented
}

// Log persists failure information when the transporter fails before
// producing a response. Implementation must update error counters and
// compute a conservative backoff. Stub returns ErrNotImplemented.
func (s *Scheduler) Log(ctx context.Context, status *Status, request *transporter.Request, err error) (*Status, error) {
	return nil, ErrNotImplemented
}

// ========================= Config Methods ===================================

const (
	defaultInterval           = 15 * time.Minute
	defaultMinInterval        = time.Minute
	defaultMaxInterval        = 12 * time.Hour
	defaultStartupGracePeriod = 5 * time.Minute
	defaultErrorBackoffStep   = time.Minute
)

// Normalize ensures all Config fields have sane defaults and bounds.
func (cfg *Config) Normalize() {
	if cfg == nil {
		return
	}
	if cfg.DefaultInterval == 0 {
		cfg.DefaultInterval = defaultInterval
	}
	if cfg.MinInterval == 0 {
		cfg.MinInterval = defaultMinInterval
	}
	if cfg.MaxInterval == 0 {
		cfg.MaxInterval = defaultMaxInterval
	}
	if cfg.StartupGracePeriod == 0 {
		cfg.StartupGracePeriod = defaultStartupGracePeriod
	}
	if cfg.ErrorBackoffStep == 0 {
		cfg.ErrorBackoffStep = defaultErrorBackoffStep
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


// ========================= Event Methods ====================================

// Ready reports whether the transporter should be invoked for this event. When
// the feed must be deferred, the method should return (false, ErrRequestNotReady)
// so callers can treat it as a no-op.
func (event *Event) Ready() (bool, error) {
	return false, ErrNotImplemented
}

// Request constructs the aggregated transporter request used to perform a
// fetch. The request encapsulates conditional validators, rate-limit guardrails,
// and a prepared http.Request derived from scheduler state.
func (event *Event) Request(ctx context.Context) (*transporter.Request, error) {
	return nil, ErrNotImplemented
}
