package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/herdworks/hyperfeed-go/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ErrNotImplemented is returned by stubbed scheduler methods until the
// production implementation is ready. Tests should assert against this sentinel
// to ensure the contract remains intact even without logic.
var ErrNotImplemented = errors.New("scheduler: not implemented")

// Scheduler coordinates feed fetch attempts using persisted state and the
// transporter. The struct intentionally avoids implementation logic at this
// stage; instead, it defines the entry points and data dependencies required to
// honor rate limits and conditional requests comprehensively.
type Scheduler struct {
	Options     Options
	Transporter TransportPlanner
	Store       StateStore
	Clock       Clock
}

// Options provides runtime configuration for a Scheduler instance.
type Options struct {
	// FeedID is the stable identifier for the feed this scheduler manages.
	FeedID string

	// DefaultInterval is the baseline polling interval when no hints are
	// available. Implementations must never schedule requests more
	// frequently than this value.
	DefaultInterval time.Duration

	// MinInterval constrains how aggressively we can poll, even when feeds
	// publish explicit TTL hints. Implementations must clamp decisions to
	// this floor to avoid overwhelming origins.
	MinInterval time.Duration

	// MaxInterval caps how stale a feed is allowed to become. Implementations
	// must clamp decisions to this ceiling even when the origin suggests very
	// long TTLs, unless the operator explicitly overrides the value.
	MaxInterval time.Duration

	// StartupGracePeriod allows newly subscribed feeds to be fetched more
	// aggressively during onboarding without violating server hints.
	StartupGracePeriod time.Duration

	// ErrorBackoffStep defines the base duration added per consecutive error.
	// Scheduler multiplies this by the failure count when no Retry-After hint
	// is available.
	ErrorBackoffStep time.Duration

	// HonorSkipDays toggles enforcement of RSS skipDays metadata.
	HonorSkipDays bool

	// HonorSkipHours toggles enforcement of RSS skipHours metadata.
	HonorSkipHours bool

	// HonorFeedTTL toggles enforcement of RSS ttl metadata.
	HonorFeedTTL bool

	// HonorCacheDirectives toggles enforcement of HTTP cache headers.
	HonorCacheDirectives bool

	// HonorRetryAfter toggles enforcement of Retry-After and related rate
	// limit hints.
	HonorRetryAfter bool

	// Honor429Backoff toggles exponential backoff when repeated 429 responses
	// are encountered even if explicit hints are missing.
	Honor429Backoff bool

	// PersistAllResponses controls whether we store metadata for successful
	// 304 responses. When false, only state-changing responses are persisted.
	PersistAllResponses bool
}

// Clock abstracts time so tests can inject deterministic clocks.
type Clock interface {
	Now() time.Time
}

// StateStore persists scheduler state between runs. Implementations may back
// this with disk, database, or in-memory storage.
type StateStore interface {
	// Load retrieves the persisted state for the scheduler's feed. Missing
	// state should be treated as a cold start.
	Load(ctx context.Context, feedID string) (*State, error)

	// Save persists the provided state atomically. Implementations must
	// ensure durability before returning.
	Save(ctx context.Context, state *State) error
}

// TransportPlanner encapsulates the transporter operations invoked by the
// scheduler. Using an interface keeps tests isolated and avoids circular
// dependencies.
type TransportPlanner interface {
	ExecutePlan(ctx context.Context, plan *transporter.RequestPlan) (*transporter.ResponseSummary, error)
	InterpretResponse(ctx context.Context, plan *transporter.RequestPlan, summary *transporter.ResponseSummary) (*transporter.PersistenceUpdate, error)
}

// State captures everything the scheduler must persist across ticks. The
// contract intentionally mirrors HTTP semantics and feed hints so we can satisfy
// the requirements outlined in the Feed Reader Behavior document.
type State struct {
	// FeedID is stored redundantly to guard against persistence mix-ups.
	FeedID string

	// CreatedAt indicates when the feed was first scheduled.
	CreatedAt time.Time

	// UpdatedAt indicates when the state was last modified.
	UpdatedAt time.Time

	// LastAttemptAt is when we most recently attempted a fetch, successful or
	// not.
	LastAttemptAt time.Time

	// LastSuccessAt is when we most recently fetched new content.
	LastSuccessAt time.Time

	// LastStatusCode stores the HTTP status code returned on the last
	// attempt, including 304 and error codes.
	LastStatusCode int

	// LastError stores a diagnostic summary for the most recent failure.
	LastError string

	// ConsecutiveErrors counts sequential failed attempts.
	ConsecutiveErrors int

	// Conditional captures validators for conditional requests.
	Conditional transporter.ConditionalRequestState

	// RateLimit captures rate limiting information supplied by the origin.
	RateLimit transporter.RateLimitState

	// CacheDirectives stores parsed HTTP cache headers.
	CacheDirectives transporter.CacheDirectiveState

	// FeedTTL holds TTL hints extracted from RSS ttl, skipHours, skipDays, or
	// equivalent metadata. Scheduler must combine this with HTTP hints.
	FeedTTL time.Duration

	// SkipDays enumerates which weekdays should be skipped, using ISO weekday
	// numbers (Monday=1). Stored as a map for quick lookups.
	SkipDays map[int]bool

	// SkipHours enumerates which hours (0-23) should be skipped.
	SkipHours map[int]bool

	// NextNotBefore stores the earliest permissible fetch time derived from
	// hints and policy. Scheduler must never schedule before this timestamp.
	NextNotBefore time.Time

	// DefaultNotBefore stores when the default interval would make the feed
	// eligible again. This is used when hints are absent or expire.
	DefaultNotBefore time.Time

	// LastContentDigest stores a digest of the last response body we actually
	// downloaded. Helps detect silent resets when validators disappear.
	LastContentDigest string

	// LastContentDigestAlgorithm identifies the algorithm used for the digest.
	LastContentDigestAlgorithm string

	// LastContentLength tracks the most recent Content-Length header.
	LastContentLength int64

	// LastETagWeak stores the weak etag if both weak and strong validators
	// were received. Allows fallback when servers change validator semantics.
	LastETagWeak string

	// ObservedItemCount records the number of entries parsed during the last
	// successful update. Useful for detecting truncated feeds.
	ObservedItemCount int

	// ObservedPublishedAt stores the newest item timestamp, allowing the
	// scheduler to reason about expected publishing cadence.
	ObservedPublishedAt time.Time

	// UserThrottleUntil stores user-imposed pause windows (manual snooze).
	UserThrottleUntil time.Time

	// Diagnostics retains arbitrary key/value pairs for debugging decisions
	// (e.g., "retry_after": "2024-12-17T15:04Z").
	Diagnostics map[string]string
}

// Decision expresses what the scheduler decided to do on a given tick.
type Decision struct {
	// Action is the high-level decision outcome.
	Action DecisionAction

	// Reason documents the primary driver for the decision (e.g.,
	// "retry-after", "cache-max-age", "manual-hold").
	Reason string

	// NotBefore is the earliest time the scheduler may revisit this feed.
	NotBefore time.Time

	// Plan describes the HTTP request to execute when ActionFetch is chosen.
	Plan *transporter.RequestPlan

	// GeneratedAt records when the decision was produced.
	GeneratedAt time.Time
}

// DecisionAction enumerates scheduler outcomes.
type DecisionAction string

const (
	// ActionFetch indicates that the feed should be fetched now.
	ActionFetch DecisionAction = "fetch"

	// ActionSkip indicates the feed should be skipped for this tick but may
	// be eligible soon.
	ActionSkip DecisionAction = "skip"

	// ActionDefer indicates the feed is under backoff until NotBefore.
	ActionDefer DecisionAction = "defer"
)

// NextDecision computes the scheduler's next action. Implementation must load
// state, honor rate-limit hints, and update persistence optimistically to avoid
// duplicate work after crashes. Stub returns ErrNotImplemented.
func (s *Scheduler) NextDecision(ctx context.Context, feed *hyperfeed.Feed) (*Decision, *State, error) {
	return nil, nil, ErrNotImplemented
}

// ApplyResult ingests a transporter response and persists updated state. The
// implementation must combine HTTP hints, feed metadata, and error handling to
// compute the next eligible fetch time. Stub returns ErrNotImplemented.
func (s *Scheduler) ApplyResult(ctx context.Context, state *State, plan *transporter.RequestPlan, summary *transporter.ResponseSummary, update *transporter.PersistenceUpdate) (*State, error) {
	return nil, ErrNotImplemented
}

// RecordFailure persists failure information when the transporter fails before
// producing a response summary. Implementation must update error counters and
// compute a conservative backoff. Stub returns ErrNotImplemented.
func (s *Scheduler) RecordFailure(ctx context.Context, state *State, plan *transporter.RequestPlan, err error) (*State, error) {
	return nil, ErrNotImplemented
}
