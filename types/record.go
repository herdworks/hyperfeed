package hyperfeed

import (
	"fmt"
)

// DatabaseRecord provides default implementations for database.DatabaseRow
type DatabaseRecord struct {
	ID string `json:"id,omitempty"` // stable HyperFeed record identifier
}

func (*DatabaseRecord) DatabaseViewIdentifier() string {
	return "id"
}
func (*DatabaseRecord) DatabaseTableIdentifier() string {
	return "id"
}
func (*DatabaseRecord) DatabaseInsertColumns() []string {
	return []string{
		"id",
	}
}
func (*DatabaseRecord) DatabaseInsertParameters(columns []string) (params []string) {
	if columns == nil {
		columns = (&DatabaseRecord{}).DatabaseInsertColumns()
	}
	for _, c := range columns {
		params = append(params, fmt.Sprintf(":%s", c))
	}
	return params
}
func (record *DatabaseRecord) DatabaseInsertValues() []any {
	return []any{
		record.ID,
	}
}
func (*DatabaseRecord) DatabaseUpdatesSet(columns []string) (updates []string) {
	if columns == nil {
		columns = (&DatabaseRecord{}).DatabaseInsertColumns()
	}
	for _, col := range columns {
		updates = append(updates, fmt.Sprintf("%s = excluded.%s", col, col))
	}
	return updates
}
