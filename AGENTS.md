# AGENTS.md

## Project-specific Instructions

<!-- Project-specific instructions take priority over general instructions -->

* Use `go fmt` instead of `gofmt` for formatting
* When necessary, use the `sqlite3` CLI tool to inspect `*.db` files in this project
* Run this project using `go run ./cmd/hyperfeed` (use `go run ./cmd/hyperfeed --help` for CLI arguments)
* To run this project with my preferred sample data, use `go run ./cmd/hyperfeed --opml example.opml`
  NOTE: if `./example.opml` isn't available, any `./*.opml` file will work.
* Always review database migration and maintenance scripts for potential idempotency issues
* This library is consumed by `../libhypertexting` (Go -> C API) and ultimately by the HyperTexting iOS/macOS app
* This library is also consumed transitively by `../hyperctl` (CLI / static site generator)
* After committing changes, downstream repos pull updates via `go get github.com/herdworks/hyperfeed@<branch>`; for local testing, downstream repos can use `replace` directives in `go.mod`
* This library depends on `../hq` for web scraping (website metadata enrichment); use the same `replace` directive pattern for local cross-repo development

<!-- See https://agents.md for more information -->
