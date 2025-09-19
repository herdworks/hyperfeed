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

Scheduler constructs a `transporter.Request` after evaluating an event. The struct contains only the distilled data the transporter needs: the prepared HTTP request, applicable conditions, attempt counters, and timing guardrails. The scheduler retains the richer context.

`Request.Valid()` performs preflight validation. It verifies guardrails (`NotBefore`, retry budgets, deadline) and returns `(false, ErrRequestNotReady)` when the request must be deferred. This allows the scheduler to differentiate no-ops from actual transport failures.

### Fetch

`Fetch(ctx, request)` runs the preflight checks and returns early when `request.Valid()` fails. Typed errors communicate the reason for the no-op (e.g., `ErrRequestNotReady`, `ErrRateLimitStale`). When the request is ready, the HTTP exchange is executed, producing a `Response`.

### Response

`Response` wraps the raw HTTP response, updated conditions, payload digests, and timing information. The scheduler interprets headers such as Retry-After or Cache-Control directly from the exposed metadata.

Supporting types capture the state that must be persisted between runs:

* `Condition` – the shared vocabulary between scheduler and transporter for conditional requests; some kinds translate into HTTP headers while others provide validation context.
* `Request` – the immutable request snapshot produced by the scheduler, including a prepared `http.Request`, conditions, and guardrail timestamps.
* `Response` – the normalized view of the HTTP exchange, including redirects, payload digests, timing data, and surfaced conditions.

## Tests

`transporter_test.go` contains skipped test skeletons that define the contract:

* `Request.Valid` must attach conditional headers and reject unsafe guardrail violations.
* `Fetch` must no-op with a typed error when the request falls inside backoff windows or violates retry budgets.
* `Response` must surface headers and timing metadata so the scheduler can derive rate-limit and cache decisions.

Once the transporter implementation is ready these tests will be turned on.
