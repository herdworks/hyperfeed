package hyperfeed

import "fmt"

// ============================ Package Types =================================

type Row interface {
	// required methods
	Identifier() string
	RowIdentifier() string
	RowTable() string
	RowColumns() []string
	RowArguments() []any
	RowUpdates() [][2]string
	RowTargets() []any
}

func RowComplianceTest(row Row) (err error) {
	if row.RowTable() == "" {
		return fmt.Errorf("RowValue.RowTable() cannot be empty")
	}
	if len(row.RowColumns()) != len(row.RowArguments()) {
		return fmt.Errorf("expected %d row arguments, got %d", len(row.RowColumns()), len(row.RowArguments()))
	}
	if len(row.RowColumns()) != len(row.RowTargets()) {
		return fmt.Errorf("expected %d row targets, got %d", len(row.RowColumns()), len(row.RowTargets()))
	}
	if len(row.RowUpdates()) >= len(row.RowColumns()) {
		return fmt.Errorf("expected at most %d row updates, got %d", len(row.RowColumns()), len(row.RowUpdates()))
	}
	return nil
}
