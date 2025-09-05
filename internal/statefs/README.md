# statefs (internal)

Filesystem-backed implementation of the `state.Store` interface.

Intent

- Keep on-disk layout and encoding private and changeable.
- Provide default implementation returned by higher-level constructors.
- Support simple advisory locking via `.lock` files.

Suggested layout

- `subscriber/` — subscriber-specific state files
- `subscriber/subscriptions.json` — derived subscription metadata
- `subscriber/subscriptions.json.lock` — advisory lock
- `scheduler/scheduler.json` — global scheduler state
- `scheduler/feeds/<feedID>.json` — per-feed scheduler state
- `scheduler/*.lock` — advisory locks for mutable files

