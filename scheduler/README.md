# HyperFeed Scheduler

The `herdworks/hyperfeed-go/scheduler` package provides request orchestration.

## Goals

* Strike a balance between feed freshness and request rates – HyperFeeds are optimized for efficiency but never stale
* Supports industry standard HTTP headers and other hints to optimize feed requests
* Supports etags, last modified metadata, cache control settings, TTLs, and skip hours/days
* Honors rate limiting and backoff/cooldown responses

