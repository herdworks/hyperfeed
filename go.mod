module github.com/herdworks/hyperfeed-go

go 1.23.5

// direct dependencies
require (
	github.com/herdworks/hypertemplates-go v0.18.0-1
	github.com/mattn/go-sqlite3 v1.14.22
	golang.org/x/net v0.43.0
)

// local development dependencies (replacements)
//
// uncomment to use local dependencies
replace github.com/herdworks/hypertemplates-go => ../hypertemplates-go

// transitive dependencies
require golang.org/x/text v0.28.0 // indirect
