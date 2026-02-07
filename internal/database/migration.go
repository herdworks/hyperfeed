package database

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type MigrationScript struct {
	DatabaseScript
}

func (MigrationScript) Root() string {
	return "sql/migration"
}

func (MigrationScript) List(version int) (scripts []MigrationScript, err error) {
	var databaseScripts []DatabaseScript
	prefix := fmt.Sprintf("%d_", version)
	match := func(path string, d fs.DirEntry) bool {
		return strings.HasSuffix(d.Name(), ".sql") && strings.HasPrefix(d.Name(), prefix)
	}

	if databaseScripts, err = (DatabaseScript{}).List(MigrationScript{}.Root(), match); err != nil {
		return nil, err
	}

	for _, script := range databaseScripts {
		scripts = append(scripts, MigrationScript{DatabaseScript: script})
	}

	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Path < scripts[j].Path
	})

	return scripts, nil
}
