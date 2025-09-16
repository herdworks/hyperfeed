# HyperFeed State

The `github.com/herdworks/hyperfeed-go/state` package manages operational runtime state
for components like the scheduler and subscriber. 
It is distinct from `storage` (feed metadata & item content) and from `cache` (item assets).

## Goals

* Deterministic URI scheme
* Portable locking system based on `.lock` files
* Separation of concerns between the State API and file system (`internal/statefs`)

## Layout

* State file root directories live alongside OPML files (i.e. `filepath.Dir(opmlPath)`)
* State files are grouped by namespace (e.g. `example.com/feed`)
* State files names may optionally include a subpath (e.g. `scheduler.json`)

```shell
state/                  # state root directory
    :namespace/         # state namespace
        :key.json       # state key
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

## State File Locking

* Lock files have no contents (empty files)
* Exclusive locks are required for all state file write operations
* State file write operations should wait (max 1000ms timeout) for lock files to be released
* Lock files are not required for read operations, but state file read operations should wait (max 1000ms timeout) until locks are released
* State file operations that timeout should retry a maximum of three times

