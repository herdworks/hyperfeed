# HyperFeed State

The `github.com/herdworks/hyperfeed-go/state` package manages operational runtime state
for components like the scheduler and subscriber. It is distinct from `storage` (which
persists normalized domain data) and from `cache` (which stores large binary assets).

Responsibilities

- Persist and retrieve lightweight, rapidly changing state (e.g., ETag, Last-Modified,
  backoff windows, retry hints, request/error counters, and subscriber bookkeeping)
- Provide simple locking around files to coordinate with external tools (e.g., OPML editors)
- Keep the public API stable while allowing internal representation changes (JSON layout, file
  structure, or backend store)

Conventions

- Namespaces: `subscriber/…`, `scheduler/…`
- Coordinator files: `subscriptions.opml`, `subscriptions.json` with `.lock` companions
- Scheduler files: `scheduler.json` (top-level) and `feeds/<feedID>.json` per feed

Root directory

- By default, the client sets the root state directory to `filepath.Dir(opmlPath)`.
- Implementations should ensure write access or return informative errors.
