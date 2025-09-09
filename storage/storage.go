package storage

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Options configures database initialization.
type Options struct {
	Path     string
	ReadOnly bool
}

// DB wraps an embedded SQL database.
type DB struct {
	*sql.DB
	path string
}

// Open initializes or opens a database at opts.Path and applies the schema.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if opts.Path == "" {
		return nil, errors.New("path is required")
	}

	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
		return nil, err
	}

	dsn := "file:" + opts.Path + "?_foreign_keys=on"
	if opts.ReadOnly {
		dsn += "&mode=ro"
	}

	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	if !opts.ReadOnly {
		if err := applySchema(ctx, conn); err != nil {
			conn.Close()
			return nil, err
		}
	}

	return &DB{DB: conn, path: opts.Path}, nil
}

// Exec is a convenience wrapper around ExecContext.
func (db *DB) Exec(ctx context.Context, query string, args ...any) error {
	_, err := db.DB.ExecContext(ctx, query, args...)
	return err
}

// Query is a convenience wrapper around QueryContext.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, query, args...)
}

// Backup copies the database file to dest.
func (db *DB) Backup(ctx context.Context, dest string) error {
	if dest == "" {
		return errors.New("dest is required")
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	src, err := os.Open(db.path)
	if err != nil {
		return err
	}
	defer src.Close()

	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dest)
}

// Vacuum compacts the database.
func (db *DB) Vacuum(ctx context.Context) error {
	return db.Exec(ctx, "VACUUM")
}

// Export writes the database file to w.
func (db *DB) Export(ctx context.Context, w io.Writer) error {
	src, err := os.Open(db.path)
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(w, src)
	return err
}

// Import replaces the database file with contents from r.
func Import(ctx context.Context, path string, r io.Reader) error {
	if path == "" {
		return errors.New("path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}
