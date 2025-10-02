package hyperfeed

import "fmt"

// ============================ Package Types =================================

type Row interface {
	// required methods
	RowTable() string
	RowColumns() []string
	RowArguments() []any
	RowUpdates() [][2]string
	RowPK() string
}

func RowComplianceTest(row Row) (err error) {
	if row.RowTable() == "" {
		return fmt.Errorf("RowValue.RowTable() cannot be empty")
	}
	if len(row.RowArguments()) != len(row.RowColumns()) {
		return fmt.Errorf("expected %d row arguments, got %d", len(row.RowColumns()), len(row.RowArguments()))
	}
	if len(row.RowUpdates()) > len(row.RowColumns()) {
		return fmt.Errorf("expected at most %d row updates, got %d", len(row.RowColumns()), len(row.RowUpdates()))
	}
	return nil
}
