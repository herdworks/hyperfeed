# HyperFeed Discovery

The `herdworks/hyperfeed-go/discovery` package provides feed discovery utilities.

## Goals

* Finding feeds should be fast, flexible, and fun!
* Flexible search API
* Support for resolving URLs
* Support for fetching HTML documents by URL
* Support for parsing HTML document metadata to identify available feeds (RSS/Atom/JSON Feed)
  - Detect `<link rel="alternate" type="application/rss+xml"|"application/atom+xml"|"application/feed+json" href=…>`
* Support for querying feed indexes (including Apple Podcasts)
* Support for social media platforms that provide RSS/Atom feeds (including YouTube, BlueSky, and Mastodon)
