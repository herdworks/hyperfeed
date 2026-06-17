# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog], and this project adheres to [Semantic Versioning].

## Unreleased

## v0.26.1 (2026-06-17)

* Fixed recommended feed discovery to return an empty JSON array when a site does not publish `/.well-known/recommendations.json`, avoiding downstream top-level `null` decode failures.

## v0.26.0 (2026-06-15)

* Updated `github.com/herdworks/hypergraph-go` to `v0.22.0` and added `RecommendedFeeds`, `RecommendedFeedsForURLs`, and `RecommendationsOPML` utilities for resolving Ghost-compatible recommendations into feed candidates and OPML subscription lists.

## v0.25.0 (2026-06-12)

* Added schema migration version 6 with searchable `websites_view.search` and `hot_hyperlinks_view.search` columns, plus flat hot-link `title` and `description` columns, so consumers can issue unified in-database `LIKE` search queries.
* Added fresh-vs-migrated schema parity coverage for views, triggers, indexes, and table column signatures, and documented the deferred FTS5 search migration constraints.
* Changed CI to run Go tests with the `sqlite_fts5` build tag, continuously verifying FTS5-enabled builds ahead of the planned search index.
* Fixed `feeder.Tick` to close HTTP response bodies on successful and error responses.
* Fixed `Subscriber.Export` to validate an existing database connection instead of always reopening it.
* Fixed `subscriber.Unfollow` to clear feed status history when marking a feed unfollowed.

## v0.24.2 (2026-06-09)

* Fixed RSS refreshes to tolerate malformed item content by skipping unparseable show-note links and enclosure URLs, defaulting non-numeric enclosure lengths to zero, and preserving valid feed items and attachments.
* Added regression coverage for malformed show-note links, invalid enclosure URLs, and non-numeric RSS enclosure lengths.

## v0.24.1 (2026-06-09)

* Fix whitespace

## v0.24.0 (2026-06-09)

* Added schema migration version 5 with read/write `Website.Following`, `Feed.Following`, and `Item.Sponsored` fields, rebuilt view JSON output, and filter indexes for following/sponsored queries.
* Added `Post` as the timeline envelope and `subscriber.SavePost` for transactional generated-post persistence across website, feed, item, bylines, attachments, and links.
* Added `subscriber.Unfollow` as the explicit soft-delete path: it deletes the feed's items and marks the feed unfollowed, while ordinary `following=false` upserts remain non-destructive.
* Changed database initialization to apply `schema.sql` only to genuinely fresh databases, leaving existing databases to migrate through versioned scripts without first applying the latest schema.
* Changed OPML import/export and no-argument refresh to respect feed follow-state: imports create followed feeds, exports exclude unfollowed/generated feeds, and refresh skips unfollowed feeds unless passed explicitly.
* Changed generated/local `file://` feed refreshes to skip HyperGraph website scraping, preserving authored website metadata for generated feeds.
* Fixed item writes without a primary link to store a NULL `link_id` instead of the `unknown` sentinel identifier.
* Added regression coverage for fresh schema initialization, fresh-vs-migrated view parity, v4-to-v5 migrations, generated post persistence/readback, soft-delete invariants, follow-state import/export/refresh behavior, and local generated feed scrape skipping.

## v0.23.0 (2026-06-07)

* Added schema migration version 4 to rebuild the `discovery` table at the canonical shape with best-effort preservation of existing rows.
* Added top-level `occurrences` and `snooze` columns to `discovery_view`, enabling discovery visibility queries without JSON extraction.
* Added v4 migration preflight repair for stale databases missing canonical `websites.following_url`, `feeds.email`, `items.comments_url`, and `discovery` columns before SQLite validates dependent views.
* Added regression coverage for v2-to-latest migration, v3 discovery row preservation, stale discovery-column repair, and stale v2 initialization against the v4 schema.
* Changed `discovery_view` JSON output to include `origin` consistently with the canonical table.
* Removed all SQL maintenance scripts; maintenance discovery now no-ops when no embedded `sql/maintenance` directory exists, and schema repair responsibility moves to versioned migrations.

## v0.22.0 (2026-06-06)

* Added `reader` package: scheme-dispatched byte reading for feed and OPML sources. `reader.Client` returns an `*http.Client` that handles `file://` URLs (local filesystem reads with `If-Modified-Since`/mtime 304 semantics) alongside http/https; `reader.Normalize` converts bare paths and empty-scheme URLs to absolute `file://` URLs; `reader.Read` performs one-shot scheme-dispatched reads with a `MaxContentLength` cap.
* Added remote OPML support: `subscriber.LoadSubscriptionList` (new), `ImportFeeds`, and `PreviewFeeds` accept local paths, `file://` URLs, or `http(s)://` URLs, dispatched by scheme.
Import persists the subscription list and feeds, Preview persists nothing.
* Added local feed support throughout the refresh pipeline: feeds with `file://` URLs are read from disk by `feeder.Tick` (via `reader.Client`), parsed, persisted, and re-polled with mtime-based 304s — groundwork for generated feeds.
* Added a cross-scheme redirect guard to `reader.Client`: because the shared client can read both http(s) and `file://`, redirects are restricted to http/https targets so a remote feed/OPML server cannot redirect a fetch into the file transport and read arbitrary local files.
* Added a feed-scheme guard to the OPML import path (now `SubscriptionList.InitFeeds`): OPML subscription lists may only reference http/https feeds. Non-web (`file://`) outlines are skipped regardless of where the OPML was read from, so importing a subscription list can never introduce a local-file feed. Local feeds enter only via direct database access.
* Changed `SubscriptionList.Parse()` (née `Load()`) to accept pre-read OPML bytes (pure parse, no I/O); callers read bytes via the `reader` package.
* Changed `SubscriptionList.Path` (`string`, JSON `"path"`) to `SubscriptionList.URI` (`*href.Href`, JSON `"uri"`), matching the `Feed.URL` idiom. `LoadSubscriptionList` now stores the normalized URI from `reader.Normalize` (bare/relative paths become absolute `file://` URLs), so the derived identifier changes for lists previously loaded via bare paths — old `subscriptionlists` rows are orphaned; nothing references them (the `feeds.subscriptionlist_id` FK was dropped in schema version 2).
* Changed `NewFeedParser` to preserve a previously detected `Feed.Kind` on empty bodies (HTTP 304 responses no longer reset `kind` to `"unknown"`).
* Changed `Feed.WebsiteMetadata` to no-op when `HTMLURL` is unset or not http(s), so local feeds skip the hypergraph scrape.* Changed `Subscriber.PreviewFeeds` (née `LoadFeeds`) to be fully read-only: it no longer saves the subscription list as a side effect, so previewing an OPML import writes nothing to the database.
* Changed the CLI `-o, --opml` flag from `<path>` to the `<uri>` model: it now accepts a local path, `file://` URL, or `http(s)://` URL, dispatched by scheme via `Subscriber.ImportFeeds`; previously it accepted only a local filesystem path (the old `LoadFeeds` → `SubscriptionList.Load` → `opml.OPML.Load` chain read via `os.ReadFile`). Help text now describes the flag as importing the subscription list rather than refreshing it.
* Changed `opml.Outline.Validate()` to require only `text` per the OPML 2.0 spec (OPML is a general-purpose outlining format); generic outline documents now validate.
* Renamed `Subscriber.LoadSubscriptions` to `Subscriber.ImportFeeds` and `Subscriber.LoadFeeds` to `Subscriber.PreviewFeeds`: both resolve and return feeds from an OPML `uri`; 
* Renamed `SubscriptionList.Load` to `SubscriptionList.Parse`, distinguishing the pure parser from the I/O loaders.
* Renamed `opml.ParseOPML` to `opml.Parse` (signature unchanged), aligning the spec-layer parser with the `SubscriptionList.Parse` naming. Breaking for external importers of `types/opml`; internal callers were already migrated.
* Moved the `hyperfeed` CLI from the repo root (`main.go`) to `cmd/hyperfeed/`, the idiomatic layout for a library-first module: the module root is no longer `package main`, and `go build ./cmd/hyperfeed` produces a binary named `hyperfeed` (previously `go build` yielded `hyperfeed-go` unless `-o` was passed). Build/run invocations change from `go run main.go` to `go run ./cmd/hyperfeed`.
* Moved the subscription-list pattern logic from `subscriber` into `SubscriptionList`: `SubscriptionList.InitFeeds` (née `Subscriber.InitFeeds`) owns the outline→`Feed` mapping and the http/https-only feed rule, and the new `SubscriptionList.MarshalOPML` owns the `Feed`→outline export (used by `Subscriber.Export`). The OPML spec layer (`types/opml`) stays generic; subscription-list rules live on the pattern type.
* Removed `opml.Load()` and the `opml.OPML.URI` field (local-file-only); read bytes via the `reader` package and parse with `opml.OPML.Unmarshal`.
* Removed `feeder.Client`, `feeder.Config`, and `feeder.DefaultConfig`; callers construct a scheme-aware client directly via `reader.Client(reader.Config{...})`. (The dead `feeder.Config.MaxRedirects` knob is gone with it — redirect policy now lives in `reader.Client`'s cross-scheme guard.)
* Removed `opml.OPML.AddOutline` and `opml.OPML.AddSubscription` (both unused, with no replacement): AddOutline added nothing over constructing an `Outline{}` and appending, and AddSubscription was subscription-list pattern logic in the spec layer. Subscription-list documents are produced from feeds via `SubscriptionList.MarshalOPML`; any future outline-authoring API belongs on `SubscriptionList` and must enforce the http/https-only feed rule (see `InitFeeds`).
* Fixed `SubscriptionList` database read-back: the `subscriptionlists_view` emits the JSON key `uri`, but the struct tag was `json:"path"`, so `ReadRow`/`ListRecords` never populated the field. The `URI` field's `json:"uri"` tag aligns with the view.
* Fixed RSS parsing to treat item `<source url="...">` values as external link attachments.
* Fixed `SubscriptionList.Identifier()` inverted logic (recomputed the ID only when one was already set, and could nil-panic).

## 0.21.0 (2026-06-03)

* Added schema migration version 3 to recreate canonical SQL views from `schema.sql` and exercise the migration path for fresh version 2 databases.
* Added `Subscriber.Open()` for callers that need to open an existing database connection without running initialization, migrations, or maintenance.
* Added regression coverage for version 2 to 3 migrations, stale version 2 initialization repair, and comments URL view behavior.
* Changed `items_view` and `posts_view` to emit `comments_url` as a nullable string instead of passing plain URLs through SQLite `json()`.
* Changed discovery maintenance to heal stale version 2 databases by restoring the canonical discovery table and view shape, including `origin` and `snooze`.

## 0.20.0 (2026-06-03)

* Added Atom, RSS, and JSON Feed parser regression tests for related/external links versus enclosure attachments.
* Added `Item.DeduplicateAttachments()` to merge duplicate item attachments by normalized href while preserving first-seen order and filling missing metadata.
* Changed Atom parsing to ignore `rel="enclosure"` links when selecting item URLs or external link attachments, and to match enclosure links case-insensitively.
* Changed RSS parsing to treat item-level `atom:link rel="enclosure"` entries as enclosure attachments instead of external link attachments.
* Changed RSS parsing to ignore item-level `rel="enclosure"` links when selecting item URLs or external link attachments.
* Fixed `ActivityKind.String()` to return the raw string value without `fmt.Sprintf`.

## 0.19.4 (2026-04-01)

* Added `activities` table and `activities_view` for persisted activity records.
* Added `hyperfeed.Activity` and `ActivityKind` with database serialization helpers.
* Changed `discovery` schema and maintenance scripts to store an `origin` field for discovery provenance.

## 0.19.3 (2026-03-31)

* Force link attachments (mimetype `text/x-url`) to `display=1`
* Add a maintenance script to fix non-displayable link attachments

## 0.19.2 (2026-03-31)

* Added `hypergraph add` test harness command (usage: `hypergraph add https://clone.fyi/rss.xml`)
* Fixed RSS item GUID permalink parser (was resolving external links as item URLs)
* Updated `github.com/herdworks/hypergraph-go` to `v0.19.1` with improved feed detection

## 0.19.1 (2026-03-30)

* Updated `github.com/herdworks/hypergraph-go` dependency to `v0.19.0`.

## 0.19.0 (2026-03-30)

Added support for Atom XMLNS extensions, improved RSS link & link attachment parsing, normalized "podcast" detection. Improved YouTube feed parsing!

* Added this changelog 🤌🏽
* Added support for XMLNS extensions in Atom feeds (e.g. for YouTube feed metadata) 
* Added `atom.Atom.Extensions` and `atom.Entry.Extensions` fields
* Added new `github.com/herdworks/hyperfeed-go/types/xmlns` package
* Added `hyperfeed.Item.ContainsPodcastMedia()` method
* Added `hyperfeed.ParseExtensionAttachments()` function for parsing attachments from RSS and Atom XML extension metadata
* Changed RSS and Atom parsers to use `media:description` extension metadata as fallback `item.Summary` (for YouTube feeds)
* Changed parsers to convert `feed.Description` contents to Markdown
* Changed parsers.go to use more consistent `parser.Bylines()` helpers
* Changed `rss.ItemLink()` to correctly return `nil` when `rss.Item.Comments` is empty;
  this fixes "RT" style RSS feeds with items containing external links only (no canonical or Comments URL).
* Changed `hyperfeed.ParseItemLinkAttachments()` to detect external links when the parent item has no URL
* Changed `hyperfeed.Attachment.Displayable()` minimum size threshold from 512px -> 480px for YouTube thumbnails
* Changed `hot_hyperlinks_view` from "3 days" window to "48 hours"
* Changed `hyperfeed.ParseLinks()` to return parser errors
* Removed `github.com/herdworks/hyperfeed-go/types/rss.Extension` type methods in favor of `xmlns.Extension` type 
* Removed `rss.IsPodcast()` and `rss.Item.IsPodcastEpisode()` in favor of `item.ContainsPodcastMedia()`
* Removed `atom.IsPodcast()` and `atom.Entry.IsPodcastEpisode()` in favor of `item.ContainsPodcastMedia()`
* Removed `jsonfeed.IsPodcast()` and `jsonfeed.Item.IsPodcastEpisode()` in favor of `item.ContainsPodcastMedia()`
* Removed XML extension types/methods/functions from `github.com/herdworks/hyperfeed-go/types/rss` package in favor of the new `xmlns` package

> **NOTE:** until now the `github.com/herdworks/hyperfeed-go` package has attempted to stay in sync with the full suite of HyperTexting packages: 
> 
> * `hypergraph-go`
> * `hyperfeed-go`
> * `hypertemplates-go`
> * `hyperproviders-go`
> * `hypertexting-go`
> * `hyperctl`
> * `libhypertexting`
> * `HyperTextingKit`
> * `hypertexting-app`
> 
> Starting with this release we will decouple the library package versions from the user-facing app versions. 


<!-- Links -->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
