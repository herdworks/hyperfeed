# HyperFeed Scheduler

The `herdworks/hyperfeed-go/scheduler` package orchestrates when to fetch each feed.

## Goals

* Strike a balance between feed freshness and request rates – HyperFeeds are optimized for efficiency but never stale
* Support every relevant HTTP hint (`Cache-Control`, `Expires`, `Age`, `Retry-After`, `X-RateLimit-*`, etc.)
* Respect feed-provided cadence hints such as RSS `skipHours`, `skipDays`, and `ttl`
* Maintain persistent state per feed so conditional requests are always used when possible
* Surface explicit decisions (`fetch`, `skip`, `defer`) together with the rationale so higher-level components can log and audit behavior

## Contracts

The scheduler evaluates each tick by constructing events and coordinating with the transporter:

* `Scheduler.Tick(ctx, feed)` – loads persisted state, evaluates all hints, and returns an `Event` capturing the scheduler's perspective for the current tick.
* `Event.Ready()` – determines whether the transporter should be invoked, returning a typed error when the event must be deferred.
* `Event.Request(ctx)` – produces a `*transporter.Request` populated with aggregated state when the event is ready to fetch.
* `Scheduler.Update(ctx, status, request, response)` – merges transporter responses into persisted state (validators, cache directives, rate-limit windows, error statistics, etc.).
* `Scheduler.Log(ctx, status, request, err)` – handles transporter errors before an HTTP response exists, computing a conservative retry window.

Each feed's persisted `State` must include:

* Last attempt timestamps, status codes, and raw headers.
* Last successful fetch metadata (entity tags, last-modified, content digests).
* Rate limiting information (Retry-After, X-RateLimit headers, exponential backoff counters).
* Cache directives (max-age, expires, age) translated into absolute `NotBefore` timestamps.
* Feed cadence hints (`skipDays`, `skipHours`, `ttl`) translated into guardrails.
* Operator overrides and diagnostic breadcrumbs (e.g., manual snooze windows).

## Persistence Requirements

State must be updated atomically so crashes do not lead to duplicate fetches.
When an event schedules a fetch, the resulting `transporter.Request` (prepared HTTP request, guardrail timestamps, aggregated snapshots) must be persisted before the HTTP call is launched. After completion, `Apply` inspects the transporter `Response` to compute the next eligible window.

## Tests

Placeholder tests live in `scheduler_test.go`. 
Each test calls `t.Skip(...)` but spells out the required behavior:

* `Event.Ready` honors `Retry-After` and persisted backoff windows.
* `Apply` persists conditional validators after successful fetches.
* `Log` applies exponential backoff when repeated failures occur without explicit server hints.

These tests become executable once the scheduler logic is implemented.
