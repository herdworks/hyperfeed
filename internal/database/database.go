package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	_ "github.com/mattn/go-sqlite3"
)

// ============================= Package Types ================================

// Database wraps an embedded SQLite database via the database/sql interface
type Database struct {
	*sql.DB

	Path    string
	Options *Options
}

// Options configures database options
type Options struct {
	ReadOnly bool
}

// ============================ Package Variables =============================

// errors
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrDatabaseConfig = errors.New("database configuration invalid")
	// TODO: replace errors.New() and fmt.Errorf() with error variables
)

// ============================ Database Methods =================================

func (db *Database) DSN() (dsn string) {
	dsn = "file:" + db.Path + "?_foreign_keys=on"
	if db.Options != nil && db.Options.ReadOnly {
		dsn += "&mode=ro"
	}
	return dsn
}

func (db *Database) Create() (err error) {
	var file *os.File
	if len(db.Path) == 0 {
		return errors.New("Database.Path is required")
	}
	if err = os.MkdirAll(filepath.Dir(db.Path), 0o755); err != nil {
		return err
	}
	// Check if file already exists
	if _, err = os.Stat(db.Path); err == nil {
		return nil // File already exists
	} else if !os.IsNotExist(err) {
		return err // Some other error occurred
	}
	// Create an empty file to reserve the path
	if file, err = os.OpenFile(db.Path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644); err != nil {
		return err
	}
	return file.Close()
}

func (db *Database) Destroy() (err error) {
	if len(db.Path) == 0 {
		return errors.Join(ErrDatabaseConfig, errors.New("Database.Path is required"))
	}
	if err = db.Close(); err != nil {
		return err
	}
	if err = os.Remove(db.Path); os.IsNotExist(err) {
		return nil // File already doesn't exist, which is what we want
	}
	return err // this *should* be nil, but could be an error
}

func (db *Database) Initialize(ctx context.Context) (err error) {
	var schemaBytes []byte
	if schemaBytes, err = Schema(); err != nil {
		return fmt.Errorf("database: load schema: %w", err)
	}
	if _, err = db.DB.ExecContext(ctx, string(schemaBytes)); err != nil {
		return fmt.Errorf("database: apply schema: %w", err)
	}
	return nil
}

func (db *Database) Open(ctx context.Context) (err error) {
	if len(db.Path) == 0 {
		return errors.New("Database.Path is required")
	}
	if db.Options == nil {
		db.Options = &Options{ReadOnly: false}
	}
	db.DB, err = sql.Open("sqlite3", db.DSN())
	if err != nil {
		return err
	}
	if err := db.DB.PingContext(ctx); err != nil {
		return err
	}
	if err = db.Initialize(ctx); err != nil {
		return err
	}
	return nil
}

func (db *Database) Close() error {
	if db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

func (db *Database) Exists() bool {
	state, err := os.Stat(db.Path)
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
func (db *Database) Exec(ctx context.Context, query string, args ...any) error {
	_, err := db.DB.ExecContext(ctx, query, args...)
	return err
}

// Query is a convenience wrapper around QueryContext.
func (db *Database) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, query, args...)
}

// Vacuum compacts the database.
func (db *Database) Vacuum(ctx context.Context) error {
	return db.Exec(ctx, "VACUUM")
}

func (db *Database) InsertRowValue(ctx context.Context, row hyperfeed.Row) (err error) {
	_, err = db.DB.ExecContext(ctx, RowInsertStatement(row), row.RowArguments()...)
	return err
}
func (db *Database) UpsertRowValue(ctx context.Context, row hyperfeed.Row) (err error) {
	_, err = db.DB.ExecContext(ctx, RowUpsertStatement(row), row.RowArguments()...)
	return err
}

// ============================ Helper Methods ===================================
func RowInsertStatement(row hyperfeed.Row) (query string) {
	query += fmt.Sprintf("INSERT INTO %s(%s)", row.RowTable(), strings.Join(row.RowColumns(), ", "))
	query += fmt.Sprintf("VALUES(%s)", strings.Join(RowValues(row), ", "))
	query += fmt.Sprintf("ON CONFLICT(%s) DO NOTHING", row.RowPK())
	return query
}
func RowUpsertStatement(row hyperfeed.Row) (query string) {
	query += fmt.Sprintf("INSERT INTO %s(%s)", row.RowTable(), strings.Join(row.RowColumns(), ", "))
	query += fmt.Sprintf("VALUES(%s)", strings.Join(RowValues(row), ", "))
	query += fmt.Sprintf("ON CONFLICT(%s) DO UPDATE SET %s", row.RowPK(), RowUpdatesSet(row))
	return query
}
func RowValues(row hyperfeed.Row) (values []string) {
	for range row.RowColumns() {
		values = append(values, "?")
	}
	return values
}
func RowUpdatesSet(row hyperfeed.Row) string {
	var updates []string = []string{}
	for _, update := range row.RowUpdates() {
		key, value := update[0], update[1]
		updates = append(updates, fmt.Sprintf("%s = %s", key, value))
	}
	return strings.Join(updates, ", ")
}
