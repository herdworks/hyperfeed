# HyperFeed Filesystem

The `github.com/herdworks/hyperfeed-go/internal/filesystem` package implements
shared filesystem primitives used by higher-level packages (cache, state, etc.).

## Goals

* Separation of concerns between storage orchestration and domain-specific logic
* Provide basic security guard rails (prevent operations outside of the configured root URI)
* Operate on file URIs with no opinionated layout
* Serve as the backing implementation for multiple packages (cache, state, …)

## Advisory Locking

The `internal/filesystem` package will expose a policy-based advisory locking system with these goals:

* A `FileSystem` can have one or more lock policies
* A `LockPolicy` is configured with a subpath pattern and a list of operation types
* An `OperationKind` enum should be used for operation types, with values for each core CRUDL operation
* An `AdvisoryLockURI` method should be used for checking if an advisory lock is required and available;
  it should return a lockfile `*url.URL` for requests that match a `LockPolicy`, and
  it should return `nil` for requests that do not match a `LockPolicy`;
* Use `if lockfile,err = fs.AdvisoryLockURI(uri); lockfile != nil {}` to make locking conditional, keeping lock-free I/O fast
* Lockfile naming is deterministic, enabling overlapping `FileSystem` scopes with different locking policies
* Directory locks append `.lock` to directory names: `path/to/directory/` is locked with `path/to/directory.lock`
* File locks append `.lock` to file names: `path/to/file.json` is locked with `path/to/file.json.lock`
