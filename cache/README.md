# HyperFeed Cache

The `github.com/herdworks/hyperfeed-go/cache` package caches `hyperfeed.Item` asset files, including images, videos, audio, and other enclosures.
It is distinct from the `storage` (feed metadata & item content) and `state` (subscription & scheduler status) packages.

## Goals

* Deterministic URI scheme
* Age-based retention policy
* Side-effect free reads; safe concurrent access
* Separation of concerns between Cache API and file system (`internal/cachefs`)

## Layout

* Cache files are grouped by namespace (e.g. `example.com/feed`)
* Cache files are sharded using `YYYY/MM/DD` subdirectories (in UTC)
* Cache files are keyed by `hyperfeed.Item.ID`
* Cache files names may optionally include a subpath (e.g. `images/cover.png`)
* Cache files URIs are constructed deterministically (i.e. namespace, item, and filename)
* Cache files are locked at the item level

```shell
cache/                      # root cache directory
    :namespace/             # cache namespace
        :YYYY/:MM/:DD/      # cache shards (UTC)
            :itemID/        # cache key
                :filename   # cache filename
                cache.lock  # cache lockfile (item-scoped)
```
