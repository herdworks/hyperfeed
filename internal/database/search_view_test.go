package database

import (
	"context"
	"path/filepath"
	"testing"
)

// TestWebsitesViewSearchColumn covers the v6 [search] column: a website
// matches on its own text AND on the text of its FOLLOWED feeds (following
// the MKBHD YouTube channel feed makes the youtube.com website match
// "MKBHD"), while unfollowed feeds never promote their website.
func TestWebsitesViewSearchColumn(t *testing.T) {
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "search_websites.db")}
	if err := db.Destroy(); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if err := db.Open(ctx); err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close(); db.Destroy() })
	if err := db.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	for _, stmt := range []string{
		`INSERT INTO websites (id, html_url, username, name, following) VALUES ('w1', 'https://youtube.com', 'youtube.com', 'YouTube', 1);`,
		`INSERT INTO feeds (id, url, website_id, title, following) VALUES ('f1', 'https://youtube.com/feeds/mkbhd', 'w1', 'MKBHD', 1);`,
		`INSERT INTO feeds (id, url, website_id, title, following) VALUES ('f2', 'https://youtube.com/feeds/unrelated', 'w1', 'Cryptozoology Weekly', 0);`,
	} {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	match := func(pattern string) []string {
		t.Helper()
		rows, err := db.DB.QueryContext(ctx,
			`SELECT [id] FROM websites_view WHERE [following] = 1 AND [search] LIKE ? ORDER BY [id]`,
			"%"+pattern+"%")
		if err != nil {
			t.Fatalf("search %q: %v", pattern, err)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("scan search %q: %v", pattern, err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate search %q: %v", pattern, err)
		}
		return ids
	}

	// Own fields match (LIKE is ASCII-case-insensitive).
	if ids := match("YOUTUBE"); len(ids) != 1 || ids[0] != "w1" {
		t.Errorf("search YOUTUBE = %v; want [w1]", ids)
	}
	// Followed feed text promotes the website (the MKBHD case).
	if ids := match("mkbhd"); len(ids) != 1 || ids[0] != "w1" {
		t.Errorf("search mkbhd = %v; want [w1]", ids)
	}
	// Unfollowed feed text does not.
	if ids := match("cryptozoology"); len(ids) != 0 {
		t.Errorf("search cryptozoology = %v; want [] (feed f2 is unfollowed)", ids)
	}
}

// TestHotHyperlinksViewSearchColumn covers the v6 flat [title]/[description]/
// [search] columns on hot_hyperlinks_view.
func TestHotHyperlinksViewSearchColumn(t *testing.T) {
	ctx := context.Background()
	db := &Database{Path: filepath.Join("testdata", "search_hyperlinks.db")}
	if err := db.Destroy(); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if err := db.Open(ctx); err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close(); db.Destroy() })
	if err := db.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// A link is "hot" when >1 distinct websites linked it within 48h with
	// positive weight.
	for _, stmt := range []string{
		`INSERT INTO websites (id, html_url) VALUES ('w1', 'https://w1.example');`,
		`INSERT INTO websites (id, html_url) VALUES ('w2', 'https://w2.example');`,
		`INSERT INTO feeds (id, url, website_id) VALUES ('f1', 'https://w1.example/feed', 'w1');`,
		`INSERT INTO feeds (id, url, website_id) VALUES ('f2', 'https://w2.example/feed', 'w2');`,
		`INSERT INTO items (id, url, feed_id, website_id, feed_at) VALUES ('i1', 'https://w1.example/1', 'f1', 'w1', datetime('now'));`,
		`INSERT INTO items (id, url, feed_id, website_id, feed_at) VALUES ('i2', 'https://w2.example/1', 'f2', 'w2', datetime('now'));`,
		`INSERT INTO links (id, href, title, description) VALUES ('l1', 'https://example.org/story', 'Pelican Story', 'briefing about pelicans');`,
		`INSERT INTO item_links (website_id, feed_id, item_id, link_id, weight) VALUES ('w1', 'f1', 'i1', 'l1', 1.0);`,
		`INSERT INTO item_links (website_id, feed_id, item_id, link_id, weight) VALUES ('w2', 'f2', 'i2', 'l1', 1.0);`,
	} {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	var id, title, description, search string
	row := db.DB.QueryRowContext(ctx,
		`SELECT [id], [title], [description], [search] FROM hot_hyperlinks_view WHERE [search] LIKE ?`,
		"%pelican%")
	if err := row.Scan(&id, &title, &description, &search); err != nil {
		t.Fatalf("hot_hyperlinks_view search row: %v", err)
	}
	if id != "l1" || title != "Pelican Story" || description != "briefing about pelicans" {
		t.Errorf("unexpected hot hyperlink row: id=%q title=%q description=%q", id, title, description)
	}
	// href is part of the search blob too.
	row = db.DB.QueryRowContext(ctx,
		`SELECT [id] FROM hot_hyperlinks_view WHERE [search] LIKE ?`, "%example.org/story%")
	if err := row.Scan(&id); err != nil {
		t.Fatalf("hot_hyperlinks_view href search: %v", err)
	}
	if id != "l1" {
		t.Errorf("href search returned %q; want l1", id)
	}
}
