# HyperFeed Transporter

The `herdworks/hyperfeed-go/transporter` package manages HTTP interactions on
behalf of the scheduler. The production logic is not implemented yet; instead we
have a set of contracts that describe how HyperFeed will become a responsible
client for every feed it touches.

## Goals

* No external dependencies (only `net/http`, `x/net/html`, and related standard
  libraries).
* Always send conditional requests when validators (ETag, Last-Modified,
  Content-Digest) are available.
* Advertise supported feed types via `Accept` header, including
  `application/rss+xml`, `application/atom+xml`, and `application/feed+json`.
* Honor every rate limiting hint (Retry-After, X-RateLimit headers, 429/503
  semantics) before issuing new requests.
* Translate cache headers and feed metadata into actionable persistence updates
  for the scheduler.

## Contracts

The transporter exposes three primary methods:

* `PrepareRequest(ctx, plan)` – constructs an `http.Request` with the correct
  conditional headers and user-specified overrides.
* `ExecutePlan(ctx, plan)` – performs the HTTP exchange, respecting rate-limit
  guardrails and streaming constraints while populating a `ResponseSummary`.
* `InterpretResponse(ctx, plan, summary)` – distills cache, rate-limit, and
  validator hints into a `PersistenceUpdate` for the scheduler.

Supporting types capture the state that must be persisted between runs:

* `ConditionalRequestState` – stores ETags, Last-Modified timestamps, fallback
  digests, and metadata about the validators we rely on.
* `RateLimitState` – tracks Retry-After deadlines, X-RateLimit counters,
  consecutive error counts, and derived polling intervals.
* `CacheDirectiveState` – normalizes Cache-Control, Expires, and Age headers.
* `RequestPlan` – the immutable plan produced by the scheduler for a single
  attempt, including the rate-limit snapshot seen at scheduling time.
* `ResponseSummary` – the normalized view of the HTTP response, including
  redirects, payload digests, and timing data.
* `PersistenceUpdate` – the condensed state to persist after each attempt.

## Tests

`transporter_test.go` contains skipped test skeletons that define the contract:

* Conditional headers must be attached when validators exist.
* Plans must be rejected when a backoff window is still active.
* Retry-After headers must translate into future `NextNotBefore` timestamps.

Once the transporter implementation is ready these tests will be turned on.
