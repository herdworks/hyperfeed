# HyperFeed Database

This package implements HyperFeed's local database (SQLite).
It persists normalized subscription lists, feeds, items, and arbitrary metadata.
Frequently updated scheduler state (ETag, Last-Modified, backoff, stats) is maintained by the
`scheduler` package (see that package's README).

Database uses the [`mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3) driver, which
requires cgo and a working C compiler. Builds targeting iOS or macOS need Xcode's toolchain;
other platforms require a standard C toolchain.

## Schema

The database schema includes the following tables:

* `subscription_lists`
* `feeds`
* `items`
* `metadata`
* `feed_metadata` (join)
* `item_metadata` (join)
* `subscription_metadata` (join)

## Usage

```go
ctx := context.Background()
opts := database.Options{Path: "hyperfeed.db"}

db, err := database.Open(ctx, opts)
if err != nil {
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

## Maintenance

* `db.Backup(ctx, destPath)` copies the database to a new location.
* `db.Vacuum(ctx)` compacts the database.
* `database.Import(ctx, path, reader)` and `db.Export(ctx, writer)` move databases in and out of the process.
