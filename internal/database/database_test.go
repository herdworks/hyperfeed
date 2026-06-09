package database

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go/types"
)

func TestOpenAndSchema(t *testing.T) {
	var err error
	var ctx context.Context = context.Background()
	db := &Database{Path: filepath.Join("testdata", "test.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy test.db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open test.db: %v", err)
	}
	if err = db.Initialize(ctx); err != nil {
		t.Fatalf("initialize test.db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if version, err := db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version: %v", err)
	} else if version != SchemaVersionLatest {
		t.Fatalf("expected fresh schema version %d, got %d", SchemaVersionLatest, version)
	}

	// Insert a subscription list record to verify tables exist.
	err = db.Exec(ctx, "INSERT INTO subscriptionlists(id, uri) VALUES(?, ?);", "sub1", "https://example.com")
	if err != nil {
		t.Fatalf("insert subscription: %v", err)
	}

	rows, err := db.Query(ctx, "SELECT id FROM subscriptionlists")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected row")
	}
}

func TestMigrateV2ToLatest(t *testing.T) {
	var err error
	var version int
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "migrate_v2_latest.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy migrate_v2_latest.db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open migrate_v2_latest.db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		db.Destroy()
	})

	// Apply the current schema, strip v5 additions, and force the version back to
	// v2 so the migration pipeline exercises each historical SQL script.
	prepareLegacySchema(ctx, t, db, 2)
	if version, err = db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version: %v", err)
	} else if version != 2 {
		t.Fatalf("expected simulated schema version 2, got %d", version)
	}

	// Migrate applies all pending scripts and lands on the latest version.
	if err = db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if version, err = db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version after migrate: %v", err)
	} else if version != SchemaVersionLatest {
		t.Fatalf("expected schema version %d after migrate, got %d", SchemaVersionLatest, version)
	}

	// Seed an item with a plain-text comments URL — real-world data that
	// json()-wrapped view columns choke on ("malformed JSON" at query time).
	_, err = db.DB.ExecContext(ctx,
		"INSERT INTO items (id, feed_at, url, comments_url, title) VALUES ('item-c', '2026-06-01T00:00:00Z', 'https://example.com/1', 'https://example.com/1#comments', 'one');")
	if err != nil {
		t.Fatalf("seed item with comments_url: %v", err)
	}

	// All canonical views are queryable post-migration, evaluating every
	// column over real rows (lazy json() errors only surface at step time).
	for _, view := range canonicalViews {
		if err = scanAllRows(ctx, db, view); err != nil {
			t.Errorf("view %q query: %v", view, err)
		}
	}

	// The comments URL round-trips through the post JSON as a plain string.
	var commentsURL string
	row := db.DB.QueryRowContext(ctx, "SELECT json_extract(post, '$.item.comments_url') FROM posts_view WHERE id = 'item-c'")
	if err = row.Scan(&commentsURL); err != nil {
		t.Fatalf("posts_view comments_url: %v", err)
	}
	if commentsURL != "https://example.com/1#comments" {
		t.Errorf("expected comments_url to round-trip, got %q", commentsURL)
	}

	// Migrate is a no-op at the latest version.
	if err = db.Migrate(ctx); err != nil {
		t.Fatalf("migrate at latest version: %v", err)
	}
}

func TestMigrateV3DiscoveryPreservesRowsAndAddsViewColumns(t *testing.T) {
	var err error
	var version int
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "migrate_v3_discovery.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy migrate_v3_discovery.db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open migrate_v3_discovery.db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		db.Destroy()
	})

	prepareLegacySchema(ctx, t, db, 2)
	applyEmbeddedSQL(ctx, t, db, "sql/migration/3_from_2.sql")
	if version, err = db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version: %v", err)
	} else if version != 3 {
		t.Fatalf("expected schema version 3 before v4 migration, got %d", version)
	}

	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO discovery (
			id, created_at, updated_at, origin, url, html_url, username, occurrences, snooze
		) VALUES (
			'disc-1', '2026-06-01T00:00:00Z', '2026-06-02T00:00:00Z', 'unit-test',
			'https://example.com/feed.xml', 'https://example.com', 'example', 7, 42
		);
	`)
	if err != nil {
		t.Fatalf("seed discovery: %v", err)
	}

	if err = db.Migrate(ctx); err != nil {
		t.Fatalf("migrate v3 discovery: %v", err)
	}
	if version, err = db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version after migrate: %v", err)
	} else if version != SchemaVersionLatest {
		t.Fatalf("expected schema version %d after migrate, got %d", SchemaVersionLatest, version)
	}

	var origin string
	var occurrences, snooze, jsonOccurrences, jsonSnooze int
	row := db.DB.QueryRowContext(ctx, `
		SELECT
			occurrences,
			snooze,
			json_extract(discovery, '$.origin'),
			json_extract(discovery, '$.occurrences'),
			json_extract(discovery, '$.snooze')
		FROM discovery_view
		WHERE id = 'disc-1'
	`)
	if err = row.Scan(&occurrences, &snooze, &origin, &jsonOccurrences, &jsonSnooze); err != nil {
		t.Fatalf("discovery_view row: %v", err)
	}
	if occurrences != 7 || snooze != 42 || jsonOccurrences != 7 || jsonSnooze != 42 {
		t.Fatalf("expected occurrences/snooze 7/42, got columns %d/%d JSON %d/%d", occurrences, snooze, jsonOccurrences, jsonSnooze)
	}
	if origin != "unit-test" {
		t.Fatalf("expected origin to round-trip, got %q", origin)
	}
}

func TestMigrateV3StaleDiscoveryAddsMissingColumns(t *testing.T) {
	var err error
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "migrate_v3_stale_discovery.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy migrate_v3_stale_discovery.db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open migrate_v3_stale_discovery.db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		db.Destroy()
	})

	prepareLegacySchema(ctx, t, db, 2)
	applyEmbeddedSQL(ctx, t, db, "sql/migration/3_from_2.sql")
	if _, err = db.DB.ExecContext(ctx, `
		INSERT INTO discovery (id, url, html_url, username, occurrences)
		VALUES ('disc-stale', 'https://example.org/feed.xml', 'https://example.org', 'example-org', 3);
	`); err != nil {
		t.Fatalf("seed stale discovery: %v", err)
	}
	for _, statement := range []string{
		"DROP VIEW IF EXISTS discovery_view;",
		"ALTER TABLE discovery DROP COLUMN snooze;",
		"ALTER TABLE discovery DROP COLUMN origin;",
	} {
		if _, err = db.DB.ExecContext(ctx, statement); err != nil {
			t.Fatalf("stale discovery shape %q: %v", statement, err)
		}
	}

	if err = db.Migrate(ctx); err != nil {
		t.Fatalf("migrate stale v3 discovery: %v", err)
	}

	for _, tc := range []struct {
		table  string
		column string
	}{
		{table: "discovery", column: "origin"},
		{table: "discovery", column: "snooze"},
	} {
		assertTableColumn(ctx, t, db, tc.table, tc.column)
	}

	var occurrences, snooze, jsonSnooze int
	row := db.DB.QueryRowContext(ctx, `
		SELECT occurrences, snooze, json_extract(discovery, '$.snooze')
		FROM discovery_view
		WHERE id = 'disc-stale'
	`)
	if err = row.Scan(&occurrences, &snooze, &jsonSnooze); err != nil {
		t.Fatalf("stale discovery_view row: %v", err)
	}
	if occurrences != 3 || snooze != 0 || jsonSnooze != 0 {
		t.Fatalf("expected stale discovery occurrences/snooze 3/0, got columns %d/%d JSON %d", occurrences, snooze, jsonSnooze)
	}
}

func applyEmbeddedSQL(ctx context.Context, t *testing.T, db *Database, path string) {
	t.Helper()
	sqlBytes, err := db.Embedded(path)
	if err != nil {
		t.Fatalf("%s load: %v", path, err)
	}
	if _, err = db.DB.ExecContext(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("%s apply: %v", path, err)
	}
}

func prepareLegacySchema(ctx context.Context, t *testing.T, db *Database, version int) {
	t.Helper()
	applyEmbeddedSQL(ctx, t, db, "sql/schema/schema.sql")
	stripV5Columns(ctx, t, db)
	forceSchemaVersion(ctx, t, db, version)
}

func stripV5Columns(ctx context.Context, t *testing.T, db *Database) {
	t.Helper()
	for _, stmt := range []string{
		// responses_view depends on posts_view; DROP COLUMN runs a full-schema
		// integrity check, so dependent views must go first.
		"DROP VIEW IF EXISTS responses_view;",
		"DROP VIEW IF EXISTS posts_view;",
		"DROP VIEW IF EXISTS items_view;",
		"DROP VIEW IF EXISTS feeds_view;",
		"DROP VIEW IF EXISTS websites_view;",
		"DROP INDEX IF EXISTS websites_following_index;",
		"DROP INDEX IF EXISTS feeds_following_index;",
		"DROP INDEX IF EXISTS feeds_website_id_following_index;",
		"DROP INDEX IF EXISTS items_sponsored_index;",
		"ALTER TABLE websites DROP COLUMN following;",
		"ALTER TABLE feeds DROP COLUMN following;",
		"ALTER TABLE items DROP COLUMN sponsored;",
	} {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("strip v5 schema %q: %v", stmt, err)
		}
	}
}

func dropCanonicalViews(ctx context.Context, t *testing.T, db *Database) {
	t.Helper()
	for _, view := range []string{
		"responses_view",
		"posts_view",
		"items_view",
		"feeds_view",
		"websites_view",
		"attachments_view",
		"bylines_view",
		"hot_hyperlinks_view",
		"links_view",
		"discovery_view",
		"activities_view",
		"subscriptionlists_view",
	} {
		if _, err := db.DB.ExecContext(ctx, "DROP VIEW IF EXISTS ["+view+"];"); err != nil {
			t.Fatalf("drop view %q: %v", view, err)
		}
	}
}

func forceSchemaVersion(ctx context.Context, t *testing.T, db *Database, version int) {
	t.Helper()
	if _, err := db.DB.ExecContext(ctx, `DROP TRIGGER IF EXISTS hyperfeed_schema_update_trigger;`); err != nil {
		t.Fatalf("drop schema update trigger: %v", err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE hyperfeed_schema SET version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1;`, version); err != nil {
		t.Fatalf("force schema version %d: %v", version, err)
	}
	if _, err := db.DB.ExecContext(ctx, `
		CREATE TRIGGER IF NOT EXISTS hyperfeed_schema_update_trigger
		BEFORE UPDATE ON hyperfeed_schema
		FOR EACH ROW
		BEGIN
			SELECT CASE
				WHEN NEW.version != OLD.version + 1
					THEN RAISE(ABORT, 'hyperfeed_schema must increment by 1')
			END;
		END;
	`); err != nil {
		t.Fatalf("restore schema update trigger: %v", err)
	}
}

// scanAllRows steps through every row of a view with all columns selected.
// SQLite evaluates view expressions (including json()) at step time, so this
// surfaces lazy errors like "malformed JSON" that count(*) queries skip.
// TestMigrateV4ToV5 exercises the real v4→v5 migration on a v4-shaped DB
// (following/sponsored absent): 5_from_4.sql must add the columns, backfill
// websites.following from feeds, recreate views, and round-trip the rebuilt JSON.
func TestMigrateV4ToV5(t *testing.T) {
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "migrate_v4_to_v5.db")}
	if err := db.Destroy(); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if err := db.Open(ctx); err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close(); db.Destroy() })

	// Apply the latest schema, then strip the v5 additions back to a v4 shape so
	// 5_from_4.sql must add following/sponsored itself.
	prepareLegacySchema(ctx, t, db, 4)

	// Seed v4-shape rows (no following/sponsored): w1 has a feed (-> following=1),
	// w2 is an orphan website (-> following=0), one item under f1/w1.
	for _, stmt := range []string{
		`INSERT INTO websites (id, html_url) VALUES ('w1', 'https://w1.example');`,
		`INSERT INTO websites (id, html_url) VALUES ('w2', 'https://w2.example');`,
		`INSERT INTO feeds (id, url, website_id) VALUES ('f1', 'https://w1.example/feed', 'w1');`,
		`INSERT INTO items (id, url, feed_id, website_id, published_at, added_at, updated_at, feed_at) VALUES ('i1', 'https://w1.example/1', 'f1', 'w1', '2025-09-09T19:00:00Z', '2025-09-09T19:00:00Z', '2025-09-09T19:00:00Z', '2025-09-09T19:00:00Z');`,
	} {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate v4->v5: %v", err)
	}
	if v, err := db.SchemaVersion(ctx); err != nil || v != SchemaVersionLatest {
		t.Fatalf("schema version = %d (err %v); want %d", v, err, SchemaVersionLatest)
	}

	assertTableColumn(ctx, t, db, "websites", "following")
	assertTableColumn(ctx, t, db, "feeds", "following")
	assertTableColumn(ctx, t, db, "items", "sponsored")

	q := func(query string, args ...any) int {
		var n int
		if err := db.DB.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		return n
	}
	// Backfill: website with a followed feed -> 1; orphan website -> 0.
	if got := q(`SELECT following FROM websites WHERE id = 'w1';`); got != 1 {
		t.Errorf("w1 (has a feed) following = %d; want 1", got)
	}
	if got := q(`SELECT following FROM websites WHERE id = 'w2';`); got != 0 {
		t.Errorf("w2 (orphan) following = %d; want 0", got)
	}

	// Lazy-view guard + JSON round-trip of the v5 columns through the rebuilt views.
	// responses_view was dropped for the v4 simulation and isn't recreated by the
	// v5 migration (it doesn't reference following/sponsored; guarded elsewhere).
	for _, view := range canonicalViews {
		if view == "responses_view" {
			continue
		}
		if err := scanAllRows(ctx, db, view); err != nil {
			t.Errorf("scan %s: %v", view, err)
		}
	}
	if got := q(`SELECT json_extract([website], '$.following') FROM websites_view WHERE id = 'w1';`); got != 1 {
		t.Errorf("websites_view w1 $.following = %d; want 1", got)
	}
	if got := q(`SELECT json_extract([feed], '$.following') FROM feeds_view WHERE id = 'f1';`); got != 1 {
		t.Errorf("feeds_view f1 $.following = %d; want 1", got)
	}
	if got := q(`SELECT json_extract([item], '$.sponsored') FROM items_view WHERE id = 'i1';`); got != 0 {
		t.Errorf("items_view i1 $.sponsored = %d; want 0", got)
	}
	if got := q(`SELECT json_extract([post], '$.item.sponsored') FROM posts_view WHERE id = 'i1';`); got != 0 {
		t.Errorf("posts_view i1 $.item.sponsored = %d; want 0", got)
	}
	if got := q(`SELECT json_extract([post], '$.website.following') FROM posts_view WHERE id = 'i1';`); got != 1 {
		t.Errorf("posts_view i1 $.website.following = %d; want 1", got)
	}
}

func scanAllRows(ctx context.Context, db *Database, view string) error {
	rows, err := db.Query(ctx, "SELECT * FROM "+view)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

var canonicalViews = []string{
	"subscriptionlists_view",
	"websites_view",
	"feeds_view",
	"items_view",
	"posts_view",
	"links_view",
	"hot_hyperlinks_view",
	"responses_view",
	"bylines_view",
	"attachments_view",
	"discovery_view",
	"activities_view",
}

func TestInitializeHealsStaleV2Database(t *testing.T) {
	// Simulates a stale v2 database from a build that predates the
	// maintenance-added columns (e.g. a TestFlight install that skipped the
	// maintenance era entirely): canonical columns missing, old views, no
	// hyperfeed_scripts records. Initialize must migrate to latest AND heal the
	// table shapes so every canonical view is queryable.
	var err error
	var version int
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "stale_v2.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy stale_v2.db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open stale_v2.db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		db.Destroy()
	})

	// Apply the current schema, then strip it back to the stale v2 shape.
	prepareLegacySchema(ctx, t, db, 2)
	dropCanonicalViews(ctx, t, db)
	for _, stale := range []string{
		"ALTER TABLE websites DROP COLUMN following_url;",
		"ALTER TABLE feeds DROP COLUMN email;",
		"ALTER TABLE items DROP COLUMN comments_url;",
		"ALTER TABLE discovery DROP COLUMN snooze;",
		"ALTER TABLE discovery DROP COLUMN origin;",
		"DELETE FROM hyperfeed_scripts;",
	} {
		if _, err = db.DB.ExecContext(ctx, stale); err != nil {
			t.Fatalf("stale shape %q: %v", stale, err)
		}
	}

	// Initialize runs schema + migrate + maintenance — the launch-time path.
	if err = db.Initialize(ctx); err != nil {
		t.Fatalf("initialize stale v2: %v", err)
	}
	if version, err = db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version: %v", err)
	} else if version != SchemaVersionLatest {
		t.Fatalf("expected schema version %d, got %d", SchemaVersionLatest, version)
	}

	// The canonical columns are healed.
	for _, tc := range []struct {
		table  string
		column string
	}{
		{table: "websites", column: "following_url"},
		{table: "feeds", column: "email"},
		{table: "items", column: "comments_url"},
		{table: "discovery", column: "origin"},
		{table: "discovery", column: "snooze"},
	} {
		assertTableColumn(ctx, t, db, tc.table, tc.column)
	}

	// Every canonical view is queryable (lazy view validation requires
	// stepping through real rows, not just count(*)).
	for _, view := range canonicalViews {
		if err = scanAllRows(ctx, db, view); err != nil {
			t.Errorf("view %q query after heal: %v", view, err)
		}
	}
}

func assertTableColumn(ctx context.Context, t *testing.T, db *Database, table, column string) {
	t.Helper()
	var count int
	row := db.DB.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name = ?", table, column)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("pragma_table_info(%q): %v", table, err)
	}
	if count != 1 {
		t.Errorf("expected column %s.%s", table, column)
	}
}

func TestVacuum(t *testing.T) {
	var err error
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "test.db")}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err = db.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Vacuum(ctx); err != nil {
		t.Fatalf("vacuum: %v", err)
	}
}

func TestInsertFeedsFromOPML(t *testing.T) {
	var err error
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "feeds.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err = db.Initialize(ctx); err != nil {
		t.Fatalf("initialize db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Parse OPML test data through the owning pattern type
	// (hyperfeed.SubscriptionList), so this test exercises the same
	// outline→Feed path — including the http/https-only guard — as production
	// imports, rather than reimplementing the mapping at the database layer.
	opmlBytes, err := os.ReadFile(filepath.Join("testdata", "nnw_default_subscriptions.opml"))
	if err != nil {
		t.Fatalf("read opml: %v", err)
	}
	var subscriptions hyperfeed.SubscriptionList
	if err := subscriptions.Parse(opmlBytes); err != nil {
		t.Fatalf("parse opml: %v", err)
	}
	feeds, err := subscriptions.InitFeeds(io.Discard)
	if err != nil {
		t.Fatalf("init feeds: %v", err)
	}
	if len(feeds) != 14 {
		t.Fatalf("expected 14 feeds; got %d", len(feeds))
	}
	for _, feed := range feeds {
		// Use URL as ID to guarantee uniqueness in this test
		if err := db.Exec(ctx, `INSERT INTO feeds(id, url, title) VALUES(?, ?, ?)`, feed.URL.String(), feed.URL.String(), feed.Title); err != nil {
			t.Fatalf("insert feed %q: %v", feed.Title, err)
		}
	}

	// Verify we inserted 14 feeds
	rows, err := db.Query(ctx, "SELECT COUNT(*) FROM feeds")
	if err != nil {
		t.Fatalf("count feeds: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no count row returned")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("scan count: %v", err)
	}
	if count != 14 {
		t.Fatalf("expected 14 feeds; got %d", count)
	}
}

func TestDatabaseRecordCRUD(t *testing.T) {
	var err error
	var ctx context.Context = context.Background()
	var db *Database = &Database{Path: filepath.Join("testdata", "crud.db")}
	ts, _ := time.Parse(time.RFC3339, "2026-01-28T22:00:00Z")
	tests := []struct {
		name   string
		writer DatabaseRow
		reader DatabaseRow
		err    error
	}{
		{
			name: "SubscriptionList record",
			writer: &hyperfeed.SubscriptionList{
				URI:       &href.Href{Scheme: "file", Path: "/data/following.opml"},
				CreatedAt: ts.UTC(),
			},
			reader: &hyperfeed.SubscriptionList{
				URI: &href.Href{Scheme: "file", Path: "/data/following.opml"},
			},
		},
		{
			name:   "Website record",
			writer: &hyperfeed.Website{Entity: hypergraph.Entity{HTMLURL: "https://example.com", FaviconURL: "https://example.com/favicon.ico"}},
			reader: &hyperfeed.Website{Entity: hypergraph.Entity{HTMLURL: "https://example.com"}},
			err:    nil,
		},
		{
			name: "Feed record",
			writer: &hyperfeed.Feed{
				URL:     &href.Href{Scheme: "https", Host: "example.com", Path: "/feed"},
				HTMLURL: &href.Href{Scheme: "https", Host: "example.com"},
				Title:   "Example Feed",
				Website: hyperfeed.Website{Entity: hypergraph.Entity{HTMLURL: "https://example.com"}},
			},
			reader: &hyperfeed.Feed{
				URL:     &href.Href{Scheme: "https", Host: "example.com", Path: "/feed"},
				Website: hyperfeed.Website{Entity: hypergraph.Entity{HTMLURL: "https://example.com"}},
			},
			err: nil,
		},
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err = db.Initialize(ctx); err != nil {
		t.Fatalf("initialize db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Validate inputs
			if tc.writer == nil || tc.reader == nil {
				t.Fatal("test case writer and reader must be non-nil")
			}
			if db.RecordsMatch(tc.writer, tc.reader) {
				t.Fatalf("pre-test row and reader should differ;\n\trow:    %#v\n\treader: %#v", tc.writer, tc.reader)
			}
			// Test upsert
			if err = db.UpsertRow(ctx, nil, tc.writer); err != nil {
				t.Fatalf("insert feed: %v", err)
			}
			if err = db.UpsertRow(ctx, nil, tc.writer); err != nil {
				t.Fatalf("insert feed: %v", err)
			}
			// Test read
			if err = db.ReadRow(ctx, tc.reader); err != nil {
				t.Fatalf("read feed: %v", err)
			}
			// Validate results
			if !db.RecordsMatch(tc.writer, tc.reader) {
				t.Fatalf("mismatched records;\n\twanted: %#v\n\tgot:    %#v", tc.writer, tc.reader)
			}

		})
	}

}

func TestReadRowPopulatesProvidedStruct(t *testing.T) {
	var err error
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "readrow.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err = db.Initialize(ctx); err != nil {
		t.Fatalf("initialize db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	writer := &hyperfeed.Website{
		Entity: hypergraph.Entity{
			HTMLURL:    "https://example.com",
			FaviconURL: "https://example.com/favicon.ico",
			Name:       "Example",
		},
	}
	if err = db.UpsertRow(ctx, nil, writer); err != nil {
		t.Fatalf("insert website: %v", err)
	}

	reader := &hyperfeed.Website{
		Entity: hypergraph.Entity{
			HTMLURL: "https://example.com",
		},
	}
	if err = db.ReadRow(ctx, reader); err != nil {
		t.Fatalf("read website: %v", err)
	}

	if reader.FaviconURL != writer.FaviconURL {
		t.Fatalf("expected favicon %q, got %q", writer.FaviconURL, reader.FaviconURL)
	}
	if reader.Name != writer.Name {
		t.Fatalf("expected name %q, got %q", writer.Name, reader.Name)
	}
	if reader.ID == "" {
		t.Fatalf("expected website ID to be set")
	}
}
