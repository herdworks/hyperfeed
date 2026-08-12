# HyperFeed

HyperFeed is a hypermedia feed reading client library written in Golang.
It supports RSS feeds, Atom feeds, JSON Feeds, and OPML files (subscription lists).
HyperFeed doesn't differentiate between text-based feeds and other content types (e.g. audio-based feeds, or "podcasts").

## Features

* Support for parsing OPML files
* Support for parsing RSS (0.90–2.0), Atom (0.3/1.0), and JSON Feed (1.x)
* Support for namespaced RSS metadata, including Dublin Core, iTunes Podcasts, Google Play Podcasts, and more
* Tolerant of common feed defects (bad dates, undeclared namespaces, etc.)
* Standards compliant request management (conditional requests, respects rate limiting / backoff headers)
* Persists feed contents and metadata to a local SQLite database

## Usage

To run hyperfeed from source:

```shell
go run ./cmd/hyperfeed --opml path/to/following.opml refresh
```

Alternatively, build and run hyperfeed as a tidy ~12MB binary:

```shell
go build ./cmd/hyperfeed
./hyperfeed --opml path/to/following.opml refresh
```

Or embed HyperFeed into an existing go app:

```go
import "github.com/herdworks/hyperfeed/subscriber"

var s subscriber.Subscriber
var config subscriber.Config = subscriber.Config{DatabasePath: "./hyperfeed.db"}

if err := s.Init(ctx, &config); err != nil {
  // handle init errors
}
// ImportFeeds reads an OPML subscription list from a uri (a local path,
// file:// URL, or http(s):// URL), persists it, and returns the feeds;
// use PreviewFeeds to resolve feeds without writing anything.
feeds, err := s.ImportFeeds(ctx, "./path/to/following.opml", os.Stdout)
if err != nil {
  // handle import errors
}
if err := s.Refresh(ctx, feeds, 30, 100, os.Stdout); err != nil {
  // handle refresh errors
}
fmt.Println(s.Status.Print())
```

## How it works

### Components

HyperFeed functionality is organized into the following components (go packages):

* `herdworks/hyperfeed-go/types`: the core HyperFeed types
* `herdworks/hyperfeed-go/subscriber`: a Subscriber manages a `SubscriptionList`
* `herdworks/hyperfeed-go/feeder`: a Feeder manages a `Feed`
* `herdworks/hyperfeed-go/internal`: normalization, transport, database, cache, etc

### Types

HyperFeed packages operate on a core set of normalized [types](`herdworks/hyperfeed-go/types`), including:

* `hyperfeed.SubscriptionList` (a subscription list; typically represented by an OPML file)
* `hyperfeed.Feed` (an individual RSS, Atom, or JSONFeed feed)
* `hyperfeed.Item` (an individual item in a `Feed`)
* `hyperfeed.Attachment` (an individual `Item` attachment)
* `hyperfeed.Metadata` (metadata properties of `SubscriptionLists`, `Feeds`, `Items`, and `Attachments`)

## Control Flow

* Subscriber: the subscription manager (multiple feeds).
  * Initialized via an OPML file path and options
  * Parses OPML, normalizes subscriptions, and schedules Feeders
  * Manages OPML file changes (additions/edits/removals)

* Feeder: the feed reader (single feed).
  * Configured with a feed, an interval, and a start time;
  * Evaluates schedule state on each tick to determine when to fetch (respects rate limits/backoff).
  * Uses `transporter` to fetch, `database` to persist items, and `cache` to store assets.
  * Shares state/stats upward by communicating, not by shared mutable structures.
