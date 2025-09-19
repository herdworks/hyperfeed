# HyperFeed Scheduler

The `herdworks/hyperfeed-go/scheduler` package orchestrates when to fetch each
feed. The current implementation is stubbed out, but the contracts below define
what the production scheduler must honor in order to be "best in class" with
respect to origin load.

## Goals

* Strike a balance between feed freshness and request rates – HyperFeeds are
  optimized for efficiency but never stale.
* Support every relevant HTTP hint (Cache-Control, Expires, Age, Retry-After,
  X-RateLimit-*, etc.).
* Respect feed-provided cadence hints such as RSS `skipHours`, `skipDays`, and
  `ttl`.
* Maintain persistent state per feed so conditional requests are always used
  when possible.
* Surface explicit decisions (`fetch`, `skip`, `defer`) together with the
  rationale so higher-level components can log and audit behavior.

## Contracts

The scheduler exposes the following primary entry points:

* `NextDecision(ctx, feed)` – loads persisted state, evaluates all hints, and
  returns a `Decision` describing the next action plus a state snapshot.
* `ApplyResult(ctx, state, plan, summary, update)` – merges transporter
  responses into the persisted state (validators, cache directives, rate-limit
  windows, error statistics, etc.).
* `RecordFailure(ctx, state, plan, err)` – handles transporter errors before an
  HTTP response exists, computing a conservative retry window.

Each feed's persisted `State` must include:

* Last attempt timestamps and status codes.
* Last successful fetch metadata (entity tags, last-modified, content digests).
* Rate limiting information (Retry-After, X-RateLimit headers, exponential
  backoff counters).
* Cache directives (max-age, expires, age) translated into absolute `NotBefore`
  timestamps.
* Feed cadence hints (`skipDays`, `skipHours`, `ttl`) translated into guardrails.
* Operator overrides and diagnostic breadcrumbs (e.g., manual snooze windows).

## Persistence Requirements

State must be updated atomically so crashes do not lead to duplicate fetches.
When a decision schedules a fetch, the `RequestPlan` (URL, conditional headers,
rate-limit snapshot, deadline, retry budget) must be persisted before the HTTP
request is launched. After completion, `ApplyResult` merges the
`PersistenceUpdate` returned by the transporter to compute the next eligible
window.

## Tests

Placeholder tests live in `scheduler_test.go`. Each test calls `t.Skip(...)` but
spells out the required behavior:

* Honor `Retry-After` and persisted backoff windows.
* Persist conditional validators after successful fetches.
* Apply exponential backoff when repeated failures occur without explicit
  server hints.

These tests become executable once the scheduler logic is implemented.
