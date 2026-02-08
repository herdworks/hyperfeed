package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupOnline(t *testing.T) {
	ctx := context.Background()
	srcPath := filepath.Join("testdata", "backup_src.db")
	destPath := filepath.Join("testdata", "backup_dest.db")

	db := &Database{Path: srcPath}
	if err := db.Destroy(); err != nil {
		t.Fatalf("destroy source db: %v", err)
	}
	t.Cleanup(func() { _ = db.Destroy() })

	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove destination db: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(destPath) })

	if err := db.Open(ctx); err != nil {
		t.Fatalf("open source db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Initialize(ctx); err != nil {
		t.Fatalf("initialize source db: %v", err)
	}
	if err := db.Exec(ctx, "INSERT INTO subscriptionlists(id, uri) VALUES(?, ?);", "sub1", "https://example.com"); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if err := db.Backup(ctx, destPath); err != nil {
		t.Fatalf("backup: %v", err)
	}

	destDB := &Database{Path: destPath}
	if err := destDB.Open(ctx); err != nil {
		t.Fatalf("open destination db: %v", err)
	}
	t.Cleanup(func() { _ = destDB.Close() })

	row := destDB.DB.QueryRowContext(ctx, "SELECT id FROM subscriptionlists WHERE id = ?", "sub1")
	var id string
	if err := row.Scan(&id); err != nil {
		t.Fatalf("scan destination row: %v", err)
	}
	if id != "sub1" {
		t.Fatalf("unexpected id: %q", id)
	}
}
