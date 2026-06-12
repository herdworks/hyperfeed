# HyperFeed SQLite Migration Plan

This document defines the migration strategy for the embedded SQLite database used by HyperFeed.
It assumes forward-only migrations and integer schema versions.

## Goals

* Support deterministic, up-only migrations in minor steps.
* Preserve user data when possible.
* Allow a clean cutoff of legacy versions by removing old migration scripts.
* Provide a safe, online backup path for upgrades and rebuilds.
* Allow unversioned maintenance scripts during pre-release development.

## Versioning Model

Schema versions are integers stored in the `hyperfeed_schema` table.
The `hyperfeed_schema` table is a single-row table with `id = 1`.
New databases are created from `internal/database/sql/schema/schema.sql` and must start at `SchemaVersionLatest`.
The `hyperfeed_schema_insert_trigger` enforces the initial version equals `SchemaVersionLatest`.
The `hyperfeed_schema_update_trigger` enforces version increments by exactly one per migration.
`SchemaVersionLatest` in `internal/database/version.go` must match the version seeded by `schema.sql`.

## Migration Files

Each migration script upgrades the schema by exactly one version.
Files are applied in strictly ascending order for a single version step.
Migrations only run when the existing database version is lower than `SchemaVersionLatest`.
The canonical latest schema is defined in `internal/database/sql/schema/schema.sql`.

### File naming

Migration file names must start with the target version and an underscore.
If the current schema version is 1, migration files must start with `2_`.
Example file names are `2_add_tags.sql` and `3_split_items_content.sql`.

### Application rules

* Migrations are discovered by `SchemaVersion()+1`, not the current version.
* Each migration file set targets exactly one version step.
* At least one matching migration script must advance `hyperfeed_schema.version` by 1.
* Migration scripts should be idempotent.
* The runner wraps each migration step in a single `BEGIN IMMEDIATE` transaction.
* Migration scripts must not include transaction statements.
* If the required migration files are missing, the system should trigger a backup and rebuild.

### Migration Script Checklist

* [ ] File name starts with the target version and an underscore, and the file lives in `internal/database/sql/migration`.
* [ ] Script does not include `BEGIN`, `COMMIT`, or `ROLLBACK` statements because the runner manages transactions.
* [ ] Script is safe to re-run and uses idempotent statements or guards to prevent duplication.
* [ ] DDL uses `IF NOT EXISTS` or `IF EXISTS` where possible to avoid hard failures on re-run.
* [ ] Data backfills use narrow `WHERE` predicates to avoid rewriting unrelated rows.
* [ ] `hyperfeed_schema` is updated from the expected prior version to the target version as the final statement.
* [ ] `SchemaVersionLatest` is updated to the new target version when the migration is ready to ship.
* [ ] `schema.sql` seeds the latest version and the insert trigger enforces it for new databases.
* [ ] Script is validated on a backup or copy of the database before release.

## Maintenance Scripts

Maintenance scripts are unversioned SQL files that run during `Database.Maintenance`.
Maintenance scripts are not gated by `hyperfeed_schema.version`.
Maintenance scripts are intended for pre-release cleanup and repair work.

### File naming

Store maintenance scripts in `internal/database/sql/maintenance`.
Use descriptive names like `backfill_feed_titles.sql` or `rebuild_items_index.sql`.

### Execution rules

* Maintenance scripts run after all versioned migrations.
* Maintenance scripts should be idempotent by design.
* The runner wraps each maintenance script in a `BEGIN IMMEDIATE` transaction.
* Maintenance scripts must not include transaction statements.
* Maintenance scripts should be tracked to avoid accidental re-runs.

## Maintenance Lifecycle And Idempotency

Maintenance scripts are tracked by `hyperfeed_scripts.id`.
The `id` is the script path, and scripts must insert their own `id` when they complete.
Only run a maintenance script when its `id` is not present in the table.
To re-run a maintenance script, rename the file and treat it as a new task.
Remove maintenance scripts only after a deliberate cutoff decision.

## Cutoff Behavior

Removing old migration files defines the minimum supported schema version.
If a database is older than the minimum supported version, the system should refuse in-place migration.
The system should take an online backup and rebuild from `schema.sql`.

## Backup Integration

Backups should use SQLite's online backup API.
This is implemented in `internal/database/backup.go`.
Backups should be performed before any rebuild or destructive migration.

## Supported Change Types And Examples

The following change types are supported by the plan.
Each example is a single migration file body.

### Add-only objects

```sql
CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS tags_name_index ON tags(name);
```

### Add column with backfill

```sql
ALTER TABLE feeds
ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1));

UPDATE feeds
SET is_active = 0
WHERE json_extract(status, '$.disabled') = 1;
```

### Index changes

```sql
DROP INDEX IF EXISTS items_url_index;
CREATE INDEX IF NOT EXISTS items_url_index ON items(url);
```

### Rename table

```sql
ALTER TABLE subscriptionlists RENAME TO subscription_lists;
```

### Rename column

```sql
ALTER TABLE items RENAME COLUMN summary TO description;
```

### Remove objects

```sql
DROP VIEW IF EXISTS items_view;
DROP TRIGGER IF EXISTS items_updated_at_trigger;
DROP TABLE IF EXISTS discovery;
```

### Rebuild-required changes

```sql
CREATE TABLE items_new (
    website_id TEXT REFERENCES websites(id) ON DELETE CASCADE,
    feed_id TEXT REFERENCES feeds(id) ON DELETE CASCADE,
    id TEXT PRIMARY KEY,
    published_at TIMESTAMP,
    title TEXT
);

INSERT INTO items_new (website_id, feed_id, id, published_at, title)
SELECT website_id, feed_id, id, published_at, title
FROM items;

DROP TABLE items;
ALTER TABLE items_new RENAME TO items;

CREATE INDEX IF NOT EXISTS items_published_at_index ON items(published_at DESC);
```

### Split or merge tables

```sql
CREATE TABLE item_content (
    item_id TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    content TEXT
);

INSERT INTO item_content (item_id, content)
SELECT id, content
FROM items
WHERE content IS NOT NULL;
```

### Data backfills and transforms

```sql
UPDATE websites
SET username = LOWER(username)
WHERE username IS NOT NULL;
```

### View changes

```sql
DROP VIEW IF EXISTS items_view;
CREATE VIEW items_view AS
SELECT id, title, published_at
FROM items
ORDER BY published_at DESC;
```

### Trigger changes

```sql
DROP TRIGGER IF EXISTS items_touch_updated_at;
CREATE TRIGGER items_touch_updated_at
AFTER UPDATE ON items
BEGIN
    UPDATE items
    SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;
```

### Full-text search (planned, not yet in the schema)

An `items_fts` FTS5 index over items is planned but deliberately deferred.
Cross-platform XCFramework builds with FTS5 enabled were verified on
2026-06-10; CI and `libhypertexting/scripts/build.sh` keep `-tags
sqlite_fts5` so that capability stays continuously verified. The migration
that introduces `items_fts` must honor all of the following (learned the
hard way; do not relearn):

* `mattn/go-sqlite3` only compiles FTS5 behind the `sqlite_fts5` build tag.
  Once the schema contains FTS5 objects, EVERY binary that opens a hyperfeed
  database must build with the tag or fails with `no such module: fts5` —
  the schema change and any consumer that lacks the tag cannot ship
  separately.
* Use an external-content table (`content='items'`, `content_rowid='rowid'`;
  `items.content` is large, a contentful FTS table would roughly double the
  database). `items` has a TEXT primary key, so the index is keyed on its
  IMPLICIT rowid — see the rowid hazards below.
* Pin the tokenizer explicitly in the DDL (e.g.
  `tokenize = 'unicode61 remove_diacritics 2'`). Changing tokenizers later
  requires a full index rebuild on every device; do not inherit defaults by
  omission.
* Backfill with `INSERT INTO items_fts(items_fts) VALUES('rebuild');`
  (idempotent). `INSERT ... SELECT` with a `WHERE NOT EXISTS (SELECT 1 FROM
  items_fts)` guard does NOT work: scanning an external-content table reads
  through to the content table even when the index is empty.
* Keep the index in sync with the FTS5 'delete'-command trigger pattern, and
  guard the UPDATE trigger with both an `OF column, ...` list and a
  `WHEN old.x IS NOT new.x ...` clause — refresh cycles upsert every item on
  every run and would otherwise reindex the whole corpus each refresh.
* Rowid hazards: VACUUM and rebuild-style migrations on `items` (CREATE new
  table → INSERT SELECT → DROP → RENAME, documented above) renumber implicit
  rowids and SILENTLY corrupt the index — no test can catch it because index
  contents are not schema. `Database.Vacuum` and any rowid-disturbing
  migration must end with the `'rebuild'` command.
* Measure the migration on a production-sized database before release: the
  backfill runs in one `BEGIN IMMEDIATE` transaction on the app-launch path,
  with a WAL spike comparable to the final index size.

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
    title,
    summary,
    content,
    content='items',
    content_rowid='rowid',
    tokenize = 'unicode61 remove_diacritics 2'
);

INSERT INTO items_fts(items_fts) VALUES('rebuild');
```

## Testing

* A fresh database creates the latest schema and sets version to `SchemaVersionLatest`.
* A fresh database should not run migrations.
* A database at version N migrates to N+1 with the next migration file.
* Missing migration files trigger a backup and rebuild.
* A database with a higher schema version than the library is rejected.
* Maintenance scripts run after migrations and are recorded in `hyperfeed_scripts`.
* A maintenance script is considered complete when its `id` exists in `hyperfeed_scripts`.
* Backup paths are created consistently and are validated by opening the destination database.
