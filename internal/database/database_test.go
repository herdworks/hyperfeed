package database

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
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

func TestVacuum(t *testing.T) {
	var err error
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "test.db")}
	if err = db.Open(ctx); err != nil {
		t.Fatalf("open: %v", err)
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
	t.Cleanup(func() { db.Close() })

	// Parse OPML test data
	subscriptions := &hyperfeed.OPML{URI: &url.URL{Path: filepath.Join("testdata", "nnw_default_subscriptions.opml")}}
	if err := subscriptions.Load(); err != nil {
		t.Fatalf("load opml: %v", err)
	}
	outlines := subscriptions.Body.Outlines
	if len(outlines) != 14 {
		t.Fatalf("expected 14 outlines; got %d", len(outlines))
	}
	for _, o := range outlines {
		if o.XMLURL == "" {
			t.Fatalf("outline missing xmlUrl for title %q", o.Title)
		}
		u, err := url.Parse(o.XMLURL)
		if err != nil {
			t.Fatalf("parse xmlUrl %q: %v", o.XMLURL, err)
		}
		// Use URL as ID to guarantee uniqueness in this test
		id := u.String()
		if err := db.Exec(ctx, `INSERT INTO feeds(id, url, title) VALUES(?, ?, ?)`, id, u.String(), o.Title); err != nil {
			t.Fatalf("insert feed %q: %v", o.Title, err)
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
