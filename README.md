# HyperFeed

HyperFeed is a universal internet feed reader client written in Golang.
It supports OPML subscription lists, RSS feeds, Atom feeds, and JSON Feed.
HyperFeed doesn't differentiate between text-based feeds and other content types (e.g. audio-based feeds, or "podcasts").

## Goals

* Support for parsing RSS (0.90–2.0), Atom (0.3/1.0), and JSON Feed (1.x)
* Tolerant of common feed defects (bad dates, undeclared namespaces, etc.)
* Standards compliant request management (conditional requests, respects rate limiting / backoff headers)
* Robust filterable storage system for quick subscription navigation and offline reading

## Packages

The `herdworks/hyperfeed-go` project contains the following high-level packages:

* `herdworks/hyperfeed-go/types`: the core HyperFeed types
* `herdworks/hyperfeed-go/client`: the HyperFeed API
* `herdworks/hyperfeed-go/subscriber`: subscription management
* `herdworks/hyperfeed-go/discovery`: feed discovery utilities
* `herdworks/hyperfeed-go/scheduler`: request orchestration
* `herdworks/hyperfeed-go/transporter`: conditional request management
* `herdworks/hyperfeed-go/storage`: storage management
* `herdworks/hyperfeed-go/state`: operational runtime state for scheduler/subscriber
* `herdworks/hyperfeed-go/cache`: asset cache for images/enclosures

## Types

The `herdworks/hyperfeed-go` packages should operate on a core set of types (defined in `herdworks/hyperfeed-go/types`), including:

* `hyperfeed.SubscriptionList` (a subscription list; typically represented by an OPML file)
* `hyperfeed.Feed` (an individual RSS or Atom feed)
* `hyperfeed.Item` (an individual item in a feed)
* `hyperfeed.Metadata` (extension metadata for SubscriptionList, Feed, and Item structs)
* `hyperfeed.RSS` (types/rss.go)
* `hyperfeed.Atom` (types/atom.go)
* `hyperfeed.JSONFeed` Feed (types/jsonfeed.go)

Shared logic about common type operations will live in the corresponding type source file (e.g. `types/feed.go`), whereas package-specific operations will live in the package methods with typed receivers (e.g. `Scheduler.Enqueue(feed hyperfeed.Feed)`).

## Terminology

- Storage: durable, queryable persistence for domain entities (subscriptions, feeds, items, and extended metadata). Backed by an embedded database (e.g., SQLite).
- State: lightweight, rapidly changing operational metadata (e.g., ETag, Last-Modified, backoff windows, request/error counters, subscriber bookkeeping). Namespaced (subscriber/scheduler) and stored via the `state` package.
- Cache: large binary assets (images, enclosures) with retention policies. Read-only w.r.t. domain semantics and safe to evict.

## Control Flow

- Client: constructs the system from an OPML path and options. Defaults the root filesystem
  to `filepath.Dir(opmlPath)` and ensures write access for `storage`, `state`, and `cache`.
- Subscriber: single owner per OPML file. Parses OPML, normalizes subscriptions, persists
  initial subscription state (via `state`), and adds/removes per-feed Schedulers as the list changes.
- Scheduler: one per active feed. Runs on an interval with a configured start time; evaluates
  its state each tick to decide whether to fetch (respects rate limits/backoff). Uses `transporter`
  to fetch, `storage` to persist items, and `cache` to store assets. Shares state/stats upward by
  communicating, not by shared mutable structures.
- Orchestration: light supervision/jitter may be applied by the client or a thin orchestrator,
  but the `scheduler` package focuses solely on single‑feed scheduling semantics.
