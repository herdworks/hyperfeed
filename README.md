# HyperFeed

HyperFeed is an internet feed reader client written in Golang.
It supports OPML subscription lists, RSS feeds, and Atom feeds.

## Goals

* Support for parsing RSS (0.90–2.0) and Atom (0.3/1.0) feeds
* Tolerant of common feed defects (bad dates, undeclared namespaces, etc.)
* Standards compliant request management (conditional requests, respects rate limiting / backoff headers)
* Robust filterable storage system for quick subscription navigation and offline reading

## Packages

The `herdworks/hyperfeed-go` package contains the following high-level packages:

* `herdworks/hyperfeed-go/types`: the core HyperFeed types
* `herdworks/hyperfeed-go/client`: the HyperFeed API
* `herdworks/hyperfeed-go/subscriber`: subscription management
* `herdworks/hyperfeed-go/discovery`: feed discovery utilities
* `herdworks/hyperfeed-go/scheduler`: request orchestration
* `herdworks/hyperfeed-go/transporter`: conditional request management
* `herdworks/hyperfeed-go/storage`: storage management

## Types

The `herdworks/hyperfeed-go` packages should operate on a core set of types (defined in `herdworks/hyperfeed-go/types`), including:

* `hyperfeed.SubscriptionList` (a subscription list; typically represented by an OPML file)
* `hyperfeed.Feed` (an individual RSS or Atom feed)
* `hyperfeed.Item` (an individual item in a feed)
* `hyperfeed.Metadata` (extension metadata for SubscriptionList, Feed, and Item structs)

Shared logic about common type operations will live in the corresponding type source file (e.g. `types/feed.go`), whereas package-specific operations will live in the package methods with typed receivers (e.g. `Scheduler.Enqueue(feed hyperfeed.Feed)`).
