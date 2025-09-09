package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndSchema(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(ctx, Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Insert a subscription list record to verify tables exist.
	err = db.Exec(ctx, "INSERT INTO subscription_lists(id, uri) VALUES(?, ?)", "sub1", "https://example.com")
	if err != nil {
		t.Fatalf("insert subscription: %v", err)
	}

	rows, err := db.Query(ctx, "SELECT id FROM subscription_lists")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected row")
	}
}

func TestBackupAndExportImport(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(ctx, Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Exec(ctx, "INSERT INTO subscription_lists(id, uri) VALUES(?, ?)", "sub1", "https://example.com"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	backupPath := filepath.Join(dir, "backup.db")
	if err := db.Backup(ctx, backupPath); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("stat backup: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := db.Export(ctx, buf); err != nil {
		t.Fatalf("export: %v", err)
	}

	importPath := filepath.Join(dir, "import.db")
	if err := Import(ctx, importPath, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("import: %v", err)
	}

	db2, err := Open(ctx, Options{Path: importPath, ReadOnly: true})
	if err != nil {
		t.Fatalf("open import: %v", err)
	}
	defer db2.Close()

	rows, err := db2.Query(ctx, "SELECT COUNT(*) FROM subscription_lists")
	if err != nil {
		t.Fatalf("query import: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no rows")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestVacuum(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(ctx, Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Vacuum(ctx); err != nil {
		t.Fatalf("vacuum: %v", err)
	}
}
