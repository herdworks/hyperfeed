//go:build sqlite_fts5

package database

import (
	"database/sql"
	"testing"
)

// TestFTS5Available proves the sqlite_fts5 build tag actually compiles the
// FTS5 module in. The schema does not require FTS5 yet, but the planned
// items_fts migration does (see MIGRATION.md "Full-text search"); CI runs
// with the tag so the capability stays continuously verified ahead of it.
// This file carries the same build tag, so untagged `go test ./...` skips it.
func TestFTS5Available(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE VIRTUAL TABLE fts5_probe USING fts5(x);"); err != nil {
		t.Fatalf("FTS5 is unavailable (%v) — the sqlite_fts5 build tag is set but the module did not compile in", err)
	}
}
