package database

import "testing"

func TestIsTransaction(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{name: "empty", script: "", want: false},
		{name: "plain statement", script: "UPDATE feeds SET title = 'x';", want: false},
		{name: "leading begin", script: "BEGIN;\nUPDATE feeds SET title = 'x';\nCOMMIT;", want: true},
		{name: "begin immediate", script: "begin immediate; select 1; commit;", want: true},
		{name: "commit after statement", script: "UPDATE feeds SET title = 'x';\nCOMMIT;", want: true},
		{name: "rollback only", script: "ROLLBACK;", want: true},
		{name: "begin after comment", script: "-- preamble\nBEGIN;", want: true},
		{
			// CREATE TRIGGER bodies use BEGIN ... END mid-statement; that is
			// trigger syntax, not transaction control (see 7_from_6.sql).
			name: "trigger body begin",
			script: `CREATE TRIGGER IF NOT EXISTS items_fts_insert_trigger
AFTER INSERT ON items
BEGIN
    INSERT INTO items_fts([rowid], [title]) VALUES (NEW.[rowid], NEW.[title]);
END;`,
			want: false,
		},
		{name: "keyword in string literal", script: "INSERT INTO feeds(title) VALUES ('begin');", want: false},
		{name: "keyword in line comment", script: "-- BEGIN\nSELECT 1;", want: false},
		{name: "keyword in block comment", script: "/* COMMIT */ SELECT 1;", want: false},
		{name: "keyword in identifier quotes", script: "SELECT [begin] FROM feeds;", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			script := &DatabaseScript{Path: tc.name, Bytes: []byte(tc.script)}
			if got := script.IsTransaction(); got != tc.want {
				t.Errorf("IsTransaction(%q) = %v; want %v", tc.script, got, tc.want)
			}
		})
	}
}
