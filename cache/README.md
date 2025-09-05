# HyperFeed Cache

The `github.com/herdworks/hyperfeed-go/cache` package caches large or external feed assets
such as images, enclosures, and thumbnails. It is distinct from `storage` (normalized domain
data) and `state` (operational metadata and counters).

Goals

- Content-addressed or deterministic keys for dedupe
- Configurable retention policies (size-, age-, and reference-based)
- Side-effect free reads; safe concurrent access

Layout (suggested)

- `objects/` content-addressed blobs
- `index/` small metadata and reference counts

