package transporter

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

var (
	ErrNotImplemented = errors.New("transporter: not implemented")
	ErrRequestNotReady = errors.New("transporter: request not ready")
	ErrRequestExpired = errors.New("transporter: request deadline exceeded")
	ErrInvalidRequest = errors.New("transporter: invalid request")
)

// Transporter orchestrates outbound HTTP requests for feeds. The struct keeps
// references to the HTTP client and configuration but intentionally avoids
// performing any logic in stub form. Once implemented, Transporter will be
// responsible for applying conditional headers, honoring rate limits, and
// extracting cache hints to share with the scheduler.
type Transporter struct {
	Client  *http.Client
	Config *Config
}

// Config captures tunable knobs for HTTP transport behavior. Every
// field in this struct is persisted so the scheduler can reconstruct request
// plans deterministically across restarts.
type Config struct {
	// Timeout specifies the timeout for HTTP requests. If zero, the
	// implementation must fall back to a safe default that balances
	// resiliency with server friendliness.
	Timeout time.Duration

	// UserAgent is the value sent in the User-Agent header. It must be
	// configurable to allow operators to identify HyperFeed traffic and to
	// differentiate staged environments. Implementations must refuse to run
	// without a non-empty value.
	UserAgent string

	// AcceptHeader enumerates the feed formats we support. The transporter
	// will combine this with caller supplied overrides when negotiating
	// content types.
	AcceptHeader string

	// MaxRedirects constrains the number of redirects followed for a single
	// request. This protects origin servers from loops and prevents the
	// transporter from amplifying traffic.
	MaxRedirects int

	// DefaultBackoff defines the minimum duration to wait after transient
	// errors when the origin does not provide Retry-After or similar hints.
	DefaultBackoff time.Duration

	// MaxBodySize caps the number of bytes that will be buffered in memory
	// for a single response. Implementations must stream to disk or abort
	// once this limit is reached.
	MaxBodySize int64

	// JitterSeed allows deterministic jitter for conditional requests. The
	// transporter should use this together with feed specific state to
	// introduce randomized delays without violating rate limits.
	JitterSeed int64

	// RespectRetryAfter toggles honoring Retry-After headers (including
	// non-standard milliseconds variants). When true, the transporter must
	// refuse to execute a plan if it knows the server requested a later
	// retry.
	RespectRetryAfter bool

	// RespectCacheHeaders toggles parsing of Cache-Control, Expires, Age,
	// and related headers. When true, cache hints must be surfaced to the
	// scheduler through PersistenceUpdate.
	RespectCacheHeaders bool

	// RespectFeedHints toggles parsing of RSS skipHours, skipDays, and TTL
	// metadata. When true, transporter responses should surface these hints
	// to the scheduler so it can persist them.
	RespectFeedHints bool

	// MinTLSVersion enforces the lowest TLS version to accept when fetching
	// feeds over HTTPS. This helps maintain strong client posture while
	// still allowing overrides for legacy deployments.
	MinTLSVersion uint16
}

// Request is the immutable description of a fetch attempt. Scheduler produces
// this snapshot so the transporter can run without needing broader scheduler
// context.
type Request struct {
	// HTTPRequest is the fully constructed HTTP request, ready for dispatch.
	HTTPRequest *http.Request

	// PreparedAt records when the scheduler assembled this request.
	PreparedAt time.Time

	// NotBefore communicates the earliest time the request may run. The
	// transporter must return ErrRequestNotReady while the current time is
	// earlier than this value.
	NotBefore time.Time

	// Deadline expresses the hard cutoff for completing the exchange. The
	// transporter should map this into context timeouts.
	Deadline time.Time

	// Attempt tracks which retry attempt this request represents.
	Attempt int

	// MaxAttempts is the retry budget allocated for this tick.
	MaxAttempts int

	// SnapshotVersion carries the scheduler's view of validator/rate-limit state
	// so the transporter can detect stale snapshots.
	SnapshotVersion string

	// Conditional captures validators to include with the request.
	Conditional ConditionalRequestState

	// RateLimit captures the rate limit snapshot evaluated by the scheduler.
	RateLimit RateLimitState

	// CacheDirectives carries cache metadata derived from persisted state.
	CacheDirectives CacheDirectiveState

	// Options allows per-request overrides without mutating global
	// Options.
	Options Options

	// Diagnostics stores optional breadcrumbs explaining how the request was
	// produced. Transporter may surface these in logs or metrics.
	Diagnostics map[string]string
}

// Options holds options for individual requests. These are
// generated by the scheduler for each fetch attempt and allow per-plan
// overrides without mutating global Options.
type Options struct {
	// Headers specifies additional headers to include in the request. The
	// transporter must merge these with conditional headers derived from
	// persisted state while deduplicating case-insensitively.
	Headers map[string]string

	// EnforceFreshBody forces a re-download even when the server returns
	// 304 Not Modified. The transporter must only honor this when the plan
	// explicitly requires it (e.g., feed indicated content-length mismatch).
	EnforceFreshBody bool

	// DisableConditional toggles whether validators should be attached to
	// the request. This is used for debugging or when servers misbehave
	// with conditional headers. Implementations must default to conditional
	// requests whenever validators are available.
	DisableConditional bool
}

// ConditionalRequestState captures the validators and reference data required
// for building conditional GET requests. Scheduler persists this structure per
// feed to minimize redundant downloads.
type ConditionalRequestState struct {
	// ETag stores the last strong or weak entity tag observed. Strong etags
	// are preferred when multiple validators are available.
	ETag string

	// LastModified stores the timestamp from the Last-Modified header.
	LastModified time.Time

	// BodySignature holds a digest (e.g., SHA-256) of the response body when
	// no validators are provided. Implementations must only compute this for
	// manageable payload sizes to avoid overwhelming origin servers.
	BodySignature string

	// BodySignatureAlgorithm identifies the hashing algorithm used for the
	// BodySignature field.
	BodySignatureAlgorithm string

	// ContentLength records the Content-Length header of the last successful
	// fetch. This is used to detect suspicious changes even when validators
	// are missing.
	ContentLength int64

	// LastSeen tracks when validators were last updated. Implementations
	// should use this to expire stale validators (e.g., after a week).
	LastSeen time.Time

	// WeakValidators retains alternate validators such as weak etags or
	// x-last-modified variants encountered in the wild.
	WeakValidators []string
}

// RateLimitState captures how often we are allowed to poll an origin. The
// scheduler persists this structure across restarts.
type RateLimitState struct {
	// BackoffUntil represents the earliest time we may retry after 429/503
	// or similar responses. The transporter must set this when servers send
	// Retry-After or equivalent hints.
	BackoffUntil time.Time

	// RetryAfterDuration stores the exact duration requested by the origin
	// if supplied by Retry-After. This allows the scheduler to reapply the
	// hint after restarts without losing fidelity.
	RetryAfterDuration time.Duration

	// Remaining and Reset track RFC-standard X-RateLimit headers. The
	// transporter must capture both integer and string variants.
	Remaining int
	Reset     time.Time

	// Window identifies the logical rate limit bucket (per origin/per feed).
	Window string

	// Consecutive429 counts repeated rate limit responses so the scheduler
	// can exponentially back off even if Retry-After is missing.
	Consecutive429 int

	// Consecutive5xx counts server errors to avoid hammering struggling
	// origins.
	Consecutive5xx int

	// SuggestedPollingInterval captures hints from RSS TTL, Cache-Control
	// max-age, and other metadata that should influence scheduling.
	SuggestedPollingInterval time.Duration

	// LastStatus records the HTTP status code associated with this state.
	LastStatus int

	// HintSources retains the names of headers or specs that influenced the
	// SuggestedPollingInterval. Useful for debugging decisions.
	HintSources []string
}

// CacheDirectiveState normalizes cache metadata from HTTP headers. Scheduler
// persists it alongside RateLimitState to determine when a feed is stale.
type CacheDirectiveState struct {
	// MaxAge reflects Cache-Control: max-age in seconds.
	MaxAge time.Duration

	// SharedMaxAge reflects Cache-Control: s-maxage when provided.
	SharedMaxAge time.Duration

	// ExpiresAt stores the absolute expiry derived from Expires header.
	ExpiresAt time.Time

	// Age denotes the Age header value at the time of fetch.
	Age time.Duration

	// ETagMismatch indicates whether conditional validators failed even
	// though cache directives suggested no change. The scheduler can use
	// this to widen polling intervals carefully.
	ETagMismatch bool
}

// Response captures the outcome of executing a Request. It wraps the raw HTTP
// response along with derived metadata so callers can persist updates without
// re-parsing headers.
type Response struct {
	// Raw is the original HTTP response. It may be nil when the exchange failed
	// before headers were received.
	Raw *http.Response

	// StatusCode is copied from Raw for convenience when Raw is present.
	StatusCode int

	// Headers contains all response headers as received.
	Headers http.Header

	// Body holds payload bytes when the transporter buffers content in memory.
	Body []byte

	// ReceivedAt records when the response completed.
	ReceivedAt time.Time

	// Duration is the round-trip time from request dispatch to final byte.
	Duration time.Duration

	// Conditional captures updated validators extracted from the response.
	Conditional ConditionalRequestState

	// RateLimit captures hints observed in the response.
	RateLimit RateLimitState

	// CacheDirectives captures parsed cache headers.
	CacheDirectives CacheDirectiveState

	// BodyDigest contains a digest of the payload when downloaded.
	BodyDigest string

	// BodyDigestAlgorithm identifies the algorithm used for BodyDigest.
	BodyDigestAlgorithm string

	// RedirectChain records intermediate redirects followed during the request.
	RedirectChain []*url.URL

	// NetworkError captures transport-level failures separate from HTTP status
	// codes.
	NetworkError error
}

// PersistenceUpdate is returned after interpreting a Response. It packages the
// data that must be persisted so the scheduler can make informed decisions
// during the next tick.
type PersistenceUpdate struct {
	// Conditional contains the validators to persist for future conditional
	// requests.
	Conditional ConditionalRequestState

	// RateLimit contains the updated rate limit state, including backoff and
	// suggested polling intervals.
	RateLimit RateLimitState

	// CacheDirectives contains normalized cache hints to persist.
	CacheDirectives CacheDirectiveState

	// NextNotBefore communicates the earliest time the scheduler should poll
	// again. This value must be derived from Retry-After, cache directives,
	// and feed metadata, choosing the most conservative (latest) timestamp.
	NextNotBefore time.Time

	// LastStatus persists the last HTTP status for auditing and debugging.
	LastStatus int

	// ErrorMessage stores a human-readable summary when the fetch failed.
	ErrorMessage string
}

// ========================== Transporter Methods =============================

// Ready returns true if the Transporter is properly configured for use.
func (transporter *Transporter) Ready() bool {
	if transporter.Config == nil {
		return false
	}
	if transporter.Client == nil {
		return false
	}
	return true
}

// Init initializes the Transporter with default settings if not already set.
func (transporter *Transporter) Init() {
	if transporter.Config == nil {
		transporter.Config = &Config{}
		transporter.Config.Timeout = 15 * time.Second
		transporter.Config.UserAgent = "hyperfeed-go/transporter (+https://github.com/herdworks/hyperfeed-go)"
		transporter.Config.AcceptHeader = "application/rss+xml, application/atom+xml, application/feed+json"
		transporter.Config.MaxRedirects = 5
		transporter.Config.DefaultBackoff = time.Minute
		transporter.Config.MaxBodySize = 4 << 20 // 4 MiB default buffer
		transporter.Config.JitterSeed = 0
		transporter.Config.RespectRetryAfter = true
		transporter.Config.RespectCacheHeaders = true
		transporter.Config.RespectFeedHints = true
	}
	if transporter.Client == nil {
		transporter.Client = &http.Client{
			Timeout: transporter.Config.Timeout,
		}
	}
}

// Fetch performs the HTTP exchange for the provided request. It first runs
// preflight validation via request.Valid(); if the request is not ready, a typed
// error such as ErrRequestNotReady must be returned. Stub returns
// ErrNotImplemented until the transport logic is available.
func (t *Transporter) Fetch(ctx context.Context, request *Request) (*Response, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}
	if _, err := request.Valid(); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

// ========================= Request Methods =============================

// Valid performs transporter-side preflight checks to confirm the request can
// execute safely. Implementations must return ErrRequestNotReady when the
// request should be postponed without counting against error budgets. The stub
// returns ErrNotImplemented until real logic is added.
func (request *Request) Valid() (bool, error) {
	return false, ErrNotImplemented
}


// ========================= Response Methods ============================

// Resolve converts a Response into a PersistenceUpdate consumable by the
// scheduler. Implementations must combine HTTP hints, cache directives, and the
// originating request metadata to compute the next eligible fetch window. Stub
// returns ErrNotImplemented for now.
func (response *Response) Resolve(ctx context.Context, request *Request) (*PersistenceUpdate, error) {
	return nil, ErrNotImplemented
}
