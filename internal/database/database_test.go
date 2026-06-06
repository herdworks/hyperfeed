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

func TestMigrateV2toV3(t *testing.T) {
	var err error
	var version int
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "migrate_v2_v3.db")}
	if err = db.Destroy(); err != nil {
		t.Fatalf("destroy migrate_v2_v3.db: %v", err)
	}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open migrate_v2_v3.db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		db.Destroy()
	})

	// Apply the base schema only — fresh databases initialize at version 2.
	schemaBytes, err := db.Embedded("sql/schema/schema.sql")
	if err != nil {
		t.Fatalf("schema load: %v", err)
	}
	if _, err = db.DB.ExecContext(ctx, string(schemaBytes)); err != nil {
		t.Fatalf("schema apply: %v", err)
	}
	if version, err = db.SchemaVersion(ctx); err != nil {
		t.Fatalf("schema version: %v", err)
	} else if version != 2 {
		t.Fatalf("expected fresh schema version 2, got %d", version)
	}

	// Migrate applies 3_from_2.sql and lands on the latest version.
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

// scanAllRows steps through every row of a view with all columns selected.
// SQLite evaluates view expressions (including json()) at step time, so this
// surfaces lazy errors like "malformed JSON" that count(*) queries skip.
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
	// hyperfeed_scripts records. Initialize must migrate to v3 AND heal the
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

	// Apply the base schema, then strip it back to the stale v2 shape.
	schemaBytes, err := db.Embedded("sql/schema/schema.sql")
	if err != nil {
		t.Fatalf("schema load: %v", err)
	}
	if _, err = db.DB.ExecContext(ctx, string(schemaBytes)); err != nil {
		t.Fatalf("schema apply: %v", err)
	}
	for _, view := range canonicalViews {
		if _, err = db.DB.ExecContext(ctx, "DROP VIEW IF EXISTS "+view); err != nil {
			t.Fatalf("drop view %q: %v", view, err)
		}
	}
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

	// The maintenance-added columns are healed.
	for table, column := range map[string]string{
		"websites": "following_url",
		"feeds":    "email",
		"items":    "comments_url",
	} {
		var count int
		row := db.DB.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name = ?", table, column)
		if err = row.Scan(&count); err != nil {
			t.Fatalf("pragma_table_info(%q): %v", table, err)
		}
		if count != 1 {
			t.Errorf("expected column %s.%s after initialize", table, column)
		}
	}

	// Every canonical view is queryable (lazy view validation requires
	// stepping through real rows, not just count(*)).
	for _, view := range canonicalViews {
		if err = scanAllRows(ctx, db, view); err != nil {
			t.Errorf("view %q query after heal: %v", view, err)
		}
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
