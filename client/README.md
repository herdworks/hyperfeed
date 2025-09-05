# HyperFeed Client

The `herdworks/hyperfeed-go/client` package wires together subscriber, scheduler,
transporter, storage, state, and cache into a cohesive client.

Initialization

- `New(opmlPath string, opts Options)` constructs a client.
- Root directory defaults to `filepath.Dir(opmlPath)` and must be writable; used for
  `storage`, `state`, and `cache` subdirectories.
- Exactly one Subscriber manages a given OPML file (singleton per OPML path).

Responsibilities

- Instantiate the Subscriber and perform first-run initialization:
  - Parse OPML, normalize subscriptions, persist initial subscription state (via `state`).
  - Create/destroy per-feed Schedulers for active/inactive feeds.
- Provide lightweight orchestration:
  - Configure per-scheduler start time (epoch base) and interval (minutes).
  - Optionally apply jitter to start times to avoid thundering herds.
- Surface events and errors from Schedulers upward in a thread-safe manner.

Non-goals

- Implement single-feed scheduling logic (owned by `scheduler`).
- Perform network I/O directly (owned by `transporter`).
