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

// Store wraps an embedded SQLite database via the database/sql interface
type Store struct {
	*sql.DB

	Path    string
	Options *Options
}

// Options configures database options
type Options struct {
	ReadOnly bool
}

func (store *Store) DSN() (dsn string) {
	dsn = "file:" + store.Path + "?_foreign_keys=on"
	if store.Options != nil && store.Options.ReadOnly {
		dsn += "&mode=ro"
	}
	return dsn
}

func (store *Store) Create() (err error) {
	var file *os.File
	if len(store.Path) == 0 {
		return errors.New("Store.Path is required")
	}
	if err = os.MkdirAll(filepath.Dir(store.Path), 0o755); err != nil {
		return err
	}
	// Check if file already exists
	if _, err = os.Stat(store.Path); err == nil {
		return nil // File already exists
	} else if !os.IsNotExist(err) {
		return err // Some other error occurred
	}
	// Create an empty file to reserve the path
	if file, err = os.OpenFile(store.Path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644); err != nil {
		return err
	}
	return file.Close()
}

func (store *Store) Destroy() (err error) {
	if len(store.Path) == 0 {
		return errors.New("Store.Path is required")
	}
	if err = store.Close(); err != nil {
		return err
	}
	if err = os.Remove(store.Path); os.IsNotExist(err) {
		return nil // File already doesn't exist, which is what we want
	}
	return err // this *should* be nil, but could be an error
}

func (store *Store) Initialize(ctx context.Context) error {
	_, err := store.DB.ExecContext(ctx, schema)
	return err
}

func (store *Store) Open(ctx context.Context) (err error) {
	if len(store.Path) == 0 {
		return errors.New("Store.Path is required")
	}
	if store.Options == nil {
		store.Options = &Options{ReadOnly: false}
	}
	store.DB, err = sql.Open("sqlite3", store.DSN())
	if err != nil {
		return err
	}
	if err := store.DB.PingContext(ctx); err != nil {
		return err
	}
	if err = store.Initialize(ctx); err != nil {
		return err
	}
	return nil
}

func (store *Store) Close() error {
	if store.DB == nil {
		return nil
	}
	return store.DB.Close()
}

func (store *Store) Exists() bool {
	state, err := os.Stat(store.Path)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	if state.IsDir() {
		return false
	}
	return true
}

// Exec is a convenience wrapper around ExecContext.
func (store *Store) Exec(ctx context.Context, query string, args ...any) error {
	_, err := store.DB.ExecContext(ctx, query, args...)
	return err
}

// Query is a convenience wrapper around QueryContext.
func (store *Store) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return store.DB.QueryContext(ctx, query, args...)
}

// Backup copies the database file to dest.
// TODO: refactor to use new Store helper methods
func (store *Store) Backup(ctx context.Context, dest string) error {
	if dest == "" {
		return errors.New("destination is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	src, err := os.Open(store.Path)
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
func (store *Store) Vacuum(ctx context.Context) error {
	return store.Exec(ctx, "VACUUM")
}

// Export writes the database file to an io.Writer
// TODO:
// * add a Dump method that performs a SQLite DUMP
// * refactor this Export method to be an alias for the Dump method
func (store *Store) Export(ctx context.Context, w io.Writer) error {
	src, err := os.Open(store.Path)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

// Import replaces the database file with contents from r.
// TODO:
// * refactor as a method on Store
// * accept a file path to a SQLite dump file rather than an io.Reader
// * perform the import as a SQLite load of the dump file
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
