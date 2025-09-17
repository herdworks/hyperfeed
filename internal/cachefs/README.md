# HyperFeed CacheFS

The `github.com/herdworks/hyperfeed-go/internal/cachefs` package implements the `cache.CacheFS` interface.

## Goals

* Separation of concerns between Cache API and file system (`internal/cachefs`)
* Provide basic security guard rails (prevent operations outside of the scope of a given CacheFS root directory)
* Operates on file URIs with no concept of layout

