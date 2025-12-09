package database

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

type DatabaseRecord[T DatabaseRow] struct {
	Value T
}
type DatabaseRow interface {
	DatabaseReadable
	DatabaseWritable
	DatabaseHashable

	Identifier() string
}
type DatabaseReadable interface {
	DatabaseView() string           // VIEW name
	DatabaseViewIdentifier() string // primary key column selector
	DatabaseObject() string         // JSON object selector is a json_object(...) expression or a column name that returns JSON
}
type DatabaseWritable interface {
	DatabaseTable() string           // TABLE name
	DatabaseTableIdentifier() string // primary key column selector
	DatabaseInsertColumns() []string // columns for INSERT/UPSERT statements
	DatabaseInsertValues() []any     // values for INSERT/UPSERT statements
	DatabaseUpdatesSet() []string    // columns and placeholders for UPSERT statements
}
type DatabaseHashable interface {
	Hash() string
}

// ============================ Package Variables =============================

// errors
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrConfig         = errors.New("database configuration invalid")
	// TODO: replace errors.New() and fmt.Errorf() with error variables
)

//go:embed sql/*
var sqlFS embed.FS // embeds database/sql/* files into the binary

// ============================ Database Methods =================================

func (db *Database) DSN() (dsn string) {
	dsn = "file:" + db.Path + "?_foreign_keys=on"
	if db.Options != nil && db.Options.ReadOnly {
		dsn += "&mode=ro"
	}
	return dsn
}

func (Database) Embedded(sqlPath string) (b []byte, err error) {
	var schemaOnce sync.Once
	var sqlBytes []byte
	schemaOnce.Do(func() {
		if sqlBytes, err = sqlFS.ReadFile(sqlPath); err != nil {
			err = fmt.Errorf("database read %s: %w", sqlPath, err)
			return
		}
	})
	return sqlBytes, err
}

func (db *Database) Create() (err error) {
	var file *os.File
	fmt.Printf("database %q create\n", db.Path)
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
		return errors.Join(ErrConfig, errors.New("Database.Path is required"))
	}
	if err = db.Close(); err != nil {
		return err
	}
	if err = os.Remove(db.Path); os.IsNotExist(err) {
		return nil // File already doesn't exist, which is what we want
	}
	return err // this *should* be nil, but could be an error
}

// Configure applies connection-level configuration (e.g. PRAGMAs) to the database
func (db *Database) Configure(ctx context.Context) (err error) {
	var statement string
	statement += "PRAGMA foreign_keys = ON;"
	statement += "PRAGMA busy_timeout = 5000;"
	if err = db.Exec(ctx, statement); err != nil {
		return errors.Join(ErrConfig, fmt.Errorf("database pragma error: %w", err))
	}
	return nil
}

func (db *Database) Initialize(ctx context.Context) (err error) {
	var schemaBytes []byte
	if schemaBytes, err = db.Embedded("sql/schema.sql"); err != nil {
		return fmt.Errorf("database: load schema: %w", err)
	}
	if err := db.DB.PingContext(ctx); err != nil {
		return err
	}
	if _, err = db.DB.ExecContext(ctx, string(schemaBytes)); err != nil {
		return fmt.Errorf("database: apply schema: %w", err)
	}
	if err = db.SchemaVersioning(ctx); err != nil {
		return fmt.Errorf("database: schema versioning: %w", err)
	}
	return nil
}

func (db *Database) SchemaVersioning(ctx context.Context) (err error) {
	var versionQuery string = "INSERT OR REPLACE INTO hyperfeed (lib_version, lib_schema_version, lib_release_date, added_at) VALUES (?, ?, ?, ?);"
	if _, err = db.DB.ExecContext(
		ctx,
		versionQuery,
		hyperfeed.LibVersion,
		hyperfeed.LibSchemaVersion,
		hyperfeed.LibReleaseDate.UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("database: set version: %w", err)
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
	if err := os.MkdirAll(filepath.Dir(db.Path), 0o755); err != nil {
		return err
	}
	if db.DB, err = sql.Open("sqlite3", db.DSN()); err != nil {
		return err
	}
	if err := db.DB.PingContext(ctx); err != nil {
		return err
	}
	if err = db.Configure(ctx); err != nil {
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

// ============================ Database CRUD methods ==========================

func (db *Database) ReadRow(ctx context.Context, row DatabaseRow) (err error) {
	var query string = db.SelectStatement(row)
	var result *sql.Row = db.DB.QueryRowContext(ctx, query, row.Identifier())
	var jsonBytes []byte
	if err = result.Scan(&jsonBytes); errors.Is(err, sql.ErrNoRows) {
		return nil // no rows found is not an error for this method
	} else if err != nil {
		return fmt.Errorf("database record read: scan: %w", err)
	}
	if err = json.Unmarshal(jsonBytes, &row); err != nil {
		return fmt.Errorf("database record %T unmarshal error: %w", &row, err)
	}
	return nil
}
func (db *Database) InsertRow(ctx context.Context, row DatabaseRow) (err error) {
	_, err = db.DB.ExecContext(ctx, db.InsertStatement(row), row.DatabaseInsertValues()...)
	return err
}
func (db *Database) UpsertRow(ctx context.Context, row DatabaseRow) (err error) {
	_, err = db.DB.ExecContext(ctx, db.UpsertStatement(row), row.DatabaseInsertValues()...)
	return err
}
func (db *Database) RelateRows(ctx context.Context, table string, columns []string, values []any) (err error) {
	if len(columns) != len(values) {
		return fmt.Errorf("relation mismatch (%d values for %d columns)", len(values), len(columns))
	}
	_, err = db.DB.ExecContext(ctx, db.RelationStatement(table, columns), values...)
	return err
}

// ============================ Helper Methods ===================================
func (Database) SelectAllStatement(row DatabaseRow) (query string) {
	return fmt.Sprintf(
		"SELECT %s FROM %s;",
		row.DatabaseObject(),
		row.DatabaseView(),
	)
}

func (Database) SelectStatement(row DatabaseRow) (query string) {
	return fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = ?;",
		row.DatabaseObject(),
		row.DatabaseView(),
		row.DatabaseViewIdentifier(),
	)
}

func (Database) InsertStatement(row DatabaseRow) (query string) {
	return fmt.Sprintf(
		"INSERT OR IGNORE INTO %s(%s) VALUES(%s);",
		row.DatabaseTable(),
		strings.Join(row.DatabaseInsertColumns(), ", "),
		strings.Join(Database{}.WriteParameters(row.DatabaseInsertColumns()), ", "),
	)
}

func (Database) UpsertStatement(row DatabaseRow) (query string) {
	return fmt.Sprintf(
		"INSERT INTO %s(%s) VALUES(%s) ON CONFLICT DO UPDATE SET %s;",
		row.DatabaseTable(),
		strings.Join(row.DatabaseInsertColumns(), ", "),
		strings.Join(Database{}.WriteParameters(row.DatabaseInsertColumns()), ", "),
		strings.Join(row.DatabaseUpdatesSet(), ", "),
	)
}

func (Database) RelationStatement(table string, columns []string) string {
	var params []string
	for range columns {
		params = append(params, "?")
	}
	return fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES(%s)", table, strings.Join(columns, ", "), strings.Join(params, ", "))
}

func (Database) WriteParameters(columns []string) (params []string) {
	for range columns {
		params = append(params, "?")
	}
	return params
}

func (Database) RecordsMatch(a, b DatabaseRow) bool {
	return a.Hash() == b.Hash()
}

func ListRecords[T DatabaseRow](ctx context.Context, db *Database) (records []T, err error) {
	var kind T
	query := db.SelectAllStatement(kind)
	rows, err := db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("database record list query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var record T
		var recordBytes []byte
		if err = rows.Scan(&recordBytes); err != nil {
			return nil, fmt.Errorf("database record list scan: %w", err)
		}
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			return nil, fmt.Errorf("database record list unmarshal: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database record list rows: %w", err)
	}
	return records, nil
}
