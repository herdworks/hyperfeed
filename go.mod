module github.com/herdworks/hyperfeed-go

go 1.23.5

// direct dependencies
require (
	github.com/herdworks/hypergraph-go v0.17.4-0.20251105234809-a30335ba4c69
	github.com/herdworks/hypertemplates-go v0.18.0-1
	github.com/mattn/go-sqlite3 v1.14.22
	golang.org/x/net v0.43.0
	golang.org/x/sync v0.16.0
)

// local development dependencies (replacements)
//
// uncomment to use local dependencies
// replace github.com/herdworks/hypertemplates-go => ../hypertemplates-go
// replace github.com/herdworks/hypergraph-go => ../hypergraph-go

// transitive dependencies
require (
	github.com/andybalholm/cascadia v1.3.2 // indirect
	golang.org/x/text v0.28.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)
