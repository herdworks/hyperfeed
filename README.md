# HyperFeed

HyperFeed is a universal internet feed reader client written in Golang.
It supports OPML subscription lists, RSS feeds, Atom feeds, and JSON Feed.
HyperFeed doesn't differentiate between text-based feeds and other content types (e.g. audio-based feeds, or "podcasts").

## Goals

* Support for parsing RSS (0.90–2.0), Atom (0.3/1.0), and JSON Feed (1.x)
* Tolerant of common feed defects (bad dates, undeclared namespaces, etc.)
* Standards compliant request management (conditional requests, respects rate limiting / backoff headers)
* Robust local database for quick subscription navigation and offline reading

## Packages

HyperFeed functionality is organized into the following packages:

* `herdworks/hyperfeed-go/types`: the core HyperFeed types
* `herdworks/hyperfeed-go/subscriber`: a Subscriber manages a SubscriptionList
* `herdworks/hyperfeed-go/feeder`: a Feeder manages a Feed
* `herdworks/hyperfeed-go/internal`: transport, database, cache, etc

## Types

HyperFeed packages operate on a core set of [types](`herdworks/hyperfeed-go/types`), including:

* `hyperfeed.SubscriptionList` (a subscription list; typically represented by an OPML file)
* `hyperfeed.Feed` (an individual RSS or Atom feed)
* `hyperfeed.Item` (an individual item in a feed)
* `hyperfeed.Attachment` (an individual item attachment)
* `hyperfeed.Metadata` (extension metadata for SubscriptionList, Feed, and Item structs)
* `hyperfeed.RSS` (types/rss.go)
* `hyperfeed.Atom` (types/atom.go)
* `hyperfeed.JSONFeed` Feed (types/jsonfeed.go)

Shared logic about common type operations will live in the corresponding type source file (e.g. `types/feed.go`), whereas package-specific operations will live in the packages (e.g. `Scheduler` wraps `SubscriptionList`, `Feeder` wraps `Feed`).

## Control Flow

* Client: the hyperfeed entry point.
  * Initialized via an OPML file path and options.
  * Defaults the root filesystem to `filepath.Dir(opmlPath)`
  * Ensures write access for `database`, `state`, and `cache`
  * Configures the subscriber

* Subscriber: the subscription manager (multiple feeds).
  * One subscriber per OPML file.
  * Subscriber parses OPML, normalizes subscriptions (feeds), and syncrhonizes feed status
  * Subscriber updates OPML file with changes to subscriptions (additions/edits/removals)

* Feeder: the feed reader (single feed).
  * Configured with a feed, an interval, and a start time;
  * Evaluates schedule state on each tick to determine when to fetch (respects rate limits/backoff).
  * Uses `transporter` to fetch, `database` to persist items, and `cache` to store assets.
  * Shares state/stats upward by communicating, not by shared mutable structures.
  * NOTE: the `scheduler` package focuses solely on single‑feed scheduling semantics
