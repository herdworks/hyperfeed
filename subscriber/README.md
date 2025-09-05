# HyperFeed Subscriber

The `herdworks/hyperfeed-go/subscriber` package provides subscription lifecycle management.

## Goals

* Subscription list management
* Maintains OPML files as the source of truth
* Separates subscription list (OPML) from state (JSON) for subscription list portability
* Feed normalization and identification
* Feed aliasing, deduplication, etc
* Feed statistics (request counts, item counts, error rates, modification tracking, etc)
* Feed hybernation (automated disabling & restoration of feed due to error rates, spam, and other conditions)

## Implementation details

OPML as source of truth

- Writes and maintains `subscriptions.opml` as the canonical subscription list.
- Persists additional derived data to `subscriptions.json` (IDs, normalization, folders, stats).
- Uses lock files for coordination when external tools may edit these files:
  - `subscriptions.opml.lock`
  - `subscriptions.json.lock`

Interaction with state package

- Subscriber-specific state (e.g., last import/export times, change counters) is stored via the
  `state` package under the `subscriber/` namespace.
