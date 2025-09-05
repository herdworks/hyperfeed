# HyperFeed Scheduler

The `herdworks/hyperfeed-go/scheduler` package provides request orchestration.

## Goals

* Strike a balance between feed freshness and request rates – HyperFeeds are optimized for efficiency but never stale
* Supports industry standard HTTP headers and other hints to optimize feed requests
* Supports etags, last modified metadata, cache control settings, TTLs, and skip hours/days
* Honors rate limiting and backoff/cooldown responses

## Scope

- Single-feed scheduling semantics only. No global orchestration or jitter logic.
- Each Scheduler runs on an interval, with a configured start time (epoch base) and interval minutes.
- On each tick, evaluates state to decide whether to fetch (e.g., rate-limited or backoff → noop).
- Invokes `transporter` to fetch feed content and conditionally updates based on ETag/Last-Modified.
- Persists items via `storage`; persists assets via `cache`.
- Shares updates/errors to the owner (e.g., Subscriber/Client) via channels/callbacks rather than shared state.

Out of scope

- Creating/destroying Schedulers (owned by Subscriber/Client).
- Applying jitter or aligning intervals across feeds (owned by orchestrator/Client).
