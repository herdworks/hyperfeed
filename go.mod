module github.com/herdworks/hyperfeed

go 1.23.5

// direct dependencies
require (
	github.com/herdworks/hq v0.24.0
	github.com/mattn/go-sqlite3 v1.14.22
	golang.org/x/net v0.43.0
	golang.org/x/sync v0.16.0
)

// local development dependencies (replacements)
//
// uncomment to use local dependencies
// replace github.com/herdworks/hq => ../hq

// transitive dependencies
require (
	github.com/andybalholm/cascadia v1.3.2 // indirect
	golang.org/x/text v0.28.0 // indirect
)
