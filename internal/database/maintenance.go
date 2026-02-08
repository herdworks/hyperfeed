package database

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type MaintenanceScript struct {
	DatabaseScript
}

func (MaintenanceScript) Root() string {
	return "sql/maintenance"
}

func (MaintenanceScript) List() (scripts []MaintenanceScript, err error) {
	var databaseScripts []DatabaseScript
	match := func(path string, d fs.DirEntry) bool {
		return strings.HasSuffix(d.Name(), ".sql")
	}

	if databaseScripts, err = (DatabaseScript{}).List(MaintenanceScript{}.Root(), match); err != nil {
		return nil, err
	}

	for _, script := range databaseScripts {
		if len(bytes.TrimSpace(script.Bytes)) == 0 {
			continue
		}
		scripts = append(scripts, MaintenanceScript{DatabaseScript: script})
	}

	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Path < scripts[j].Path
	})

	return scripts, nil
}

func (script *MaintenanceScript) Applied(ctx context.Context, db *Database) (bool, error) {
	var found string
	var query string = fmt.Sprintf("SELECT id FROM [%s] WHERE id = ? LIMIT 1", script.Table())
	if db == nil || db.DB == nil {
		return false, errors.Join(ErrConfig, errors.New("database is nil"))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.DB.QueryRowContext(ctx, query, script.Path).Scan(&found); errors.Is(err, sql.ErrNoRows) {
		return false, nil // new maintenance script
	} else if err != nil {
		return false, err // query error
	}
	return true, nil // maintenance already applied
}
