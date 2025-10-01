# HyperFeed State

The `github.com/herdworks/hyperfeed-go/state` package caches subscriber and scheduler state.
It is distinct from the `storage` (feed metadata & item content) and `cache` (item assets) packages.

## Goals

* Deterministic URI scheme
* Separation of concerns between the State API and file system (`internal/filesystem`)
* Side-effect free reads; safe concurrent access

## Layout

* State file root directories live alongside OPML files (i.e. `filepath.Dir(opmlPath)`)
* State files are grouped by namespace (e.g. `example.com/feed`)
* State files names may optionally include a subpath (e.g. `example/scheduler.json`)

```shell
state/                  # state root directory
    :namespace/         # state namespace (State.URI; e.g. "example.com/07c224a0441242df3a90991fa2cfe24e10d8ca8d")
        :key.json       # state key (File.Name; e.g. "scheduler.json")
        :key.json.lock  # state lock
```

## State File URIs

State file URIs are constructed deterministically using a `Feed` and a key.

* The state file namespace domain is generated from the Feed URL

  ```go
  normalization.Domain(feed.URL) // a DNS host name
  ```

* The state file namespace feed ID is generated from the Feed URL.Path and URL.RawQuery
  
  ```go
  feedID := normalization.Identifier(normalization.Path(feed.URL)) // 40-digit hex string
  ```

* The state file key is cleaned using `normalization.CleanPath()`
  
  NOTE: _State writers should surface a descriptive error when `CleanPath` rejects a filename._

## State File Encoding

All state files are encoded as JSON.

## State File Locking Policy

* Exclusive locks are required for all state file write operations

