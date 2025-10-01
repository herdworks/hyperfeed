package storage

import (
	"embed"
	"fmt"
	"sync"
)

//go:embed sql/*
var sqlFS embed.FS

var (
	schema []byte
)

func Schema() (b []byte, err error) {
	var schemaOnce sync.Once
	if schema != nil {
		return schema, nil
	}
	schemaOnce.Do(func() {
		if schema, err = sqlFS.ReadFile("sql/schema.sql"); err != nil {
			err = fmt.Errorf("storage: read sql/schema.sql: %w", err)
			return
		}
	})
	return schema, err
}
