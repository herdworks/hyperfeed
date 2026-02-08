package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const (
	backupStepPages = 100
	backupStepDelay = 10 * time.Millisecond
)

var (
	ErrBackup = errors.New("database backup error")
)

// Backup creates an online backup using SQLite's backup API.
func (db *Database) Backup(ctx context.Context, destPath string) (err error) {
	var destDB *sql.DB
	var srcConn, destConn *sql.Conn
	if db == nil || db.DB == nil {
		return errors.Join(ErrBackup, errors.New("database is nil"))
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// validate the destination path and ensure it doesn't match the source path
	if destPath = filepath.Clean(destPath); destPath == "" {
		return errors.Join(ErrBackup, errors.New("backup destination path is required"))
	} else if destPath == filepath.Clean(db.Path) {
		return errors.Join(ErrBackup, errors.New("backup destination matches source path"))
	}

	// ensure the destination directory exists and the destination file doesn't exist to avoid overwriting an existing backup
	if err = os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return errors.Join(ErrBackup, fmt.Errorf("create backup directory: %w", err))
	}
	if _, err = os.Stat(destPath); err == nil {
		return errors.Join(ErrBackup, fmt.Errorf("backup destination %q already exists", destPath))
	} else if !os.IsNotExist(err) {
		return errors.Join(ErrBackup, fmt.Errorf("stat backup destination: %w", err))
	}

	// open the destination database with foreign keys enabled to ensure they are copied correctly
	if destDB, err = sql.Open("sqlite3", "file:"+destPath+"?_foreign_keys=on"); err != nil {
		return errors.Join(ErrBackup, fmt.Errorf("open backup destination: %w", err))
	} else {
		defer destDB.Close()
	}
	// use the same connection for the backup to ensure it sees a consistent view of the source database
	if srcConn, err = db.DB.Conn(ctx); err != nil {
		return errors.Join(ErrBackup, fmt.Errorf("open source connection: %w", err))
	} else {
		defer srcConn.Close()
	}
	// use a separate connection for the destination to avoid locking the source database during the backup
	if destConn, err = destDB.Conn(ctx); err != nil {
		return errors.Join(ErrBackup, fmt.Errorf("open destination connection: %w", err))
	} else {
		defer destConn.Close()
	}

	// execute the "online backup" using SQLite's backup API
	if err = executeBackup(ctx, srcConn, destConn); err != nil {
		return errors.Join(ErrBackup, fmt.Errorf("backup failed: %w", err))
	}

	return nil
}

func executeBackup(ctx context.Context, srcConn, destConn *sql.Conn) (err error) {
	var backup *sqlite3.SQLiteBackup
	var ok bool
	return destConn.Raw(func(d any) error {
		var dest *sqlite3.SQLiteConn
		if dest, ok = d.(*sqlite3.SQLiteConn); !ok {
			return errors.New("destination connection is not sqlite3")
		}
		return srcConn.Raw(func(s any) error {
			if src, ok := s.(*sqlite3.SQLiteConn); !ok {
				return errors.New("source connection is not sqlite3")
			} else if backup, err = dest.Backup("main", src, "main"); err != nil {
				return err
			}
			for {
				if ctxErr := ctx.Err(); ctxErr != nil {
					_ = backup.Close()
					return ctxErr
				}
				if done, err := backup.Step(backupStepPages); err != nil {
					_ = backup.Close()
					return err
				} else if done {
					break
				}
				time.Sleep(backupStepDelay)
			}
			return backup.Close()
		})
	})
}
