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
* State files names may optionally include a subpath (e.g. `scheduler/feed.json`)

```shell
state/                  # state root directory
    :namespace/         # state namespace
        :key.json       # state key
        :key.json.lock  # state lock
```
