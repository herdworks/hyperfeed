# cachefs (internal)

Filesystem-backed implementation of the `cache.Cache` interface.

Intent

- Content-addressed or deterministic keys for dedupe
- Pluggable retention policies
- Readers and writers safe for concurrent access

Suggested layout

- `objects/xx/…` — content-addressed blobs by hash prefix
- `index/` — small metadata, reference counts, pin sets

