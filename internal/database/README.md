# HyperFeed Database

This package implements HyperFeed's embedded SQLite database and migration system.
It persists subscription lists, websites, feeds, items, attachments, bylines, links, and metadata.
Runtime fetch state is stored in the JSON `status` columns on feeds and websites.

Database uses the `mattn/go-sqlite3` driver, which requires cgo and a working C compiler.
Builds targeting iOS or macOS need Xcode's toolchain, and other platforms require a standard C toolchain.

## Schema And Versioning

The latest schema lives in `internal/database/sql/schema/schema.sql`.
New databases are created from `schema.sql` and seeded at `SchemaVersionLatest`.
`SchemaVersionLatest` is defined in `internal/database/version.go` and must match the schema seed version.
The `hyperfeed_schema_insert_trigger` enforces the initial version equals `SchemaVersionLatest`.
The `hyperfeed_schema_update_trigger` enforces monotonic version increments by exactly one.

## Migrations

Migrations live in `internal/database/sql/migration` and are named `<to_version>_<short_description>.sql`.
Migrations only run when an existing database version is lower than `SchemaVersionLatest`.
Each migration set targets exactly one version step and must update `hyperfeed_schema.version` as the final statement.
The migration runner wraps each version step in a single `BEGIN IMMEDIATE` transaction.
Migration scripts must not include transaction statements of their own.
See `internal/database/MIGRATION.md` for the full policy and checklist.

## Maintenance

Maintenance scripts live in `internal/database/sql/maintenance` and run after migrations.
Maintenance scripts must be idempotent and must record themselves in `hyperfeed_scripts`.
Maintenance scripts are wrapped in a `BEGIN IMMEDIATE` transaction by the runner.

## Usage

```go
ctx := context.Background()

db := &database.Database{Path: "hyperfeed.db"}
if err := db.Open(ctx); err != nil {
    log.Fatal(err)
}
if err := db.Initialize(ctx); err != nil {
    log.Fatal(err)
}
defer db.Close()

rows, err := db.Query(ctx, `
    SELECT items.id, items.title
    FROM items
    ORDER BY published_at DESC
    LIMIT ?`, 50)
if err != nil {
    log.Fatal(err)
}
defer rows.Close()
```

## Operations

* `db.Backup(ctx, destPath)` copies the database to a new location.
* `db.Vacuum(ctx)` compacts the database.
* `db.Migrate(ctx)` applies versioned migrations when needed.
* `db.Maintenance(ctx)` runs maintenance scripts after migrations.
