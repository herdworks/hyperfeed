# HyperFeed Transporter

The `herdworks/hyperfeed-go/transporter` package manages HTTP interactions on behalf of the scheduler.

## Goals

* No external dependencies (only `net/http`, `x/net/html`, and related standard libraries).
* Always send conditional requests when validators (ETag, Last-Modified, Content-Digest) are available.
* Advertise supported feed types via `Accept` header, including `application/rss+xml`, `application/atom+xml`, and `application/feed+json`.
* Honor every rate limiting hint (Retry-After, X-RateLimit headers, 429/503 semantics) before issuing new requests.
* Translate cache headers and feed metadata into actionable persistence updates for the scheduler.

## Contracts

The transporter exposes a compact surface area built around three concepts:

* `Request` – wraps an `http.Request` together with aggregate scheduler hints such as `NotBefore`, `Deadline`, conditional validators, and rate limit snapshots.
* `Fetch(ctx, request)` – performs preflight checks via `request.Valid()` and, when ready, executes the HTTP exchange returning a `Response`.
* `Response` – wraps the raw `http.Response` plus derived metadata. A response is resolved into scheduler persistence via `response.Resolve(ctx, request)`.

### Request

Scheduler constructs a `transporter.Request` after evaluating an event. The struct contains only the distilled data the transporter needs: prepared HTTP request, conditional validators, cache directives, rate-limit snapshot, attempt counters, and guardrail timestamps. The scheduler retains the richer context.

`Request.Valid()` performs preflight validation. It verifies guardrails (`NotBefore`, retry budgets, deadline) and returns `(false, ErrRequestNotReady)` when the request must be deferred. This allows the scheduler to differentiate no-ops from actual transport failures.

### Fetch

`Fetch(ctx, request)` runs the preflight checks and returns early when `request.Valid()` fails. Typed errors communicate the reason for the no-op (e.g., `ErrRequestNotReady`, `ErrRateLimitStale`). When the request is ready, the HTTP exchange is executed, producing a `Response`.

### Response

`Response` wraps the raw HTTP response together with normalized metadata: updated validators, cache directives, rate limit hints, payload digests, and timing information. `Response.Resolve(ctx, request)` distills those hints into a `PersistenceUpdate` for the scheduler.

Supporting types capture the state that must be persisted between runs:

* `ConditionalRequestState` – stores ETags, Last-Modified timestamps, fallback digests, and metadata about the validators we rely on.
* `RateLimitState` – tracks Retry-After deadlines, X-RateLimit counters, consecutive error counts, and derived polling intervals.
* `CacheDirectiveState` – normalizes Cache-Control, Expires, and Age headers.
* `Request` – the immutable request snapshot produced by the scheduler, including a prepared `http.Request`, guardrail timestamps, and aggregated state needed for transporter preflight checks.
* `Response` – the normalized view of the HTTP exchange, including redirects, payload digests, and timing data.
* `PersistenceUpdate` – the condensed state to persist after each attempt.

## Tests

`transporter_test.go` contains skipped test skeletons that define the contract:

* `Request.Valid` must attach conditional headers and reject unsafe guardrail violations.
* `Fetch` must no-op with a typed error when the request falls inside backoff windows or violates retry budgets.
* `Response.Resolve` must translate Retry-After and cache headers into future `NextNotBefore` timestamps.

Once the transporter implementation is ready these tests will be turned on.
