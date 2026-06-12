package subscriber

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go/types"
)

func mustParse(t *testing.T, raw string) *href.Href {
	t.Helper()
	h, err := href.Parse(raw)
	if err != nil {
		t.Fatalf("href.Parse(%q) error = %v", raw, err)
	}
	return h
}

// newSponsoredPost builds a generated sponsored post: following=false on the
// feed/website, sponsored=true on the item, with one byline and one attachment.
func newSponsoredPost(t *testing.T, day string) *hyperfeed.Post {
	t.Helper()
	published, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", day, err)
	}
	return &hyperfeed.Post{
		Website: &hyperfeed.Website{
			Entity:    hypergraph.Entity{HTMLURL: "https://hypertexting.com", Name: "HyperTexting", FaviconURL: "https://hypertexting.com/favicon.png"},
			Following: false,
		},
		Feed: &hyperfeed.Feed{
			URL:       mustParse(t, "https://hypertexting.com/sponsors/feed.json"),
			HTMLURL:   mustParse(t, "https://hypertexting.com"),
			Kind:      "generated",
			Title:     "Sponsors",
			Following: false,
		},
		Item: &hyperfeed.Item{
			URL:         mustParse(t, "https://hypertexting.com/sponsors/"+day),
			Title:       "Weekly Sponsorship Program",
			Summary:     "Promote your product to HyperTexting users.",
			Sponsored:   true,
			PublishedAt: published,
			Bylines:     []hyperfeed.Byline{{Name: "Team Herd Works", Href: mustParse(t, "https://herd.works"), Email: "sales@herd.works"}},
			Attachments: []hyperfeed.Attachment{{Href: mustParse(t, "https://hypertexting.com/sponsors/cover.png"), Mimetype: "image/png", Title: "cover"}},
			Links:       []*hyperfeed.HyperLink{{Href: mustParse(t, "https://hypertexting.com/sponsors/learn-more"), Weight: 0.25}},
		},
	}
}

func newSubscriber(t *testing.T) *Subscriber {
	t.Helper()
	s := &Subscriber{}
	if err := s.Init(context.Background(), &Config{DatabasePath: filepath.Join(t.TempDir(), "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	return s
}

func count(t *testing.T, s *Subscriber, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.Database.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q error = %v", query, err)
	}
	return n
}

// TestPostSave verifies a generated Post persists its website, feed, item,
// bylines, and attachments; following=false (without purge) and sponsored=true
// stick; and a re-save is idempotent.
func TestPostSave(t *testing.T) {
	ctx := context.Background()
	s := newSubscriber(t)
	post := newSponsoredPost(t, "2025-09-09")

	if err := SavePost(ctx, s.Database, post); err != nil {
		t.Fatalf("SavePost() error = %v", err)
	}

	if got := count(t, s, `SELECT COUNT(*) FROM websites WHERE following = 0;`); got != 1 {
		t.Errorf("sponsor websites (following=0) = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM feeds WHERE following = 0;`); got != 1 {
		t.Errorf("generated feeds (following=0) = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM items WHERE sponsored = 1;`); got != 1 {
		t.Errorf("sponsored items = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM item_bylines;`); got != 1 {
		t.Errorf("item_bylines = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM item_attachments;`); got != 1 {
		t.Errorf("item_attachments = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM links;`); got != 1 {
		t.Errorf("links = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM item_links;`); got != 1 {
		t.Errorf("item_links = %d; want 1", got)
	}
	// Content assertion guards the RelateRows column→value mapping for attachments.
	var mimetype string
	if err := s.Database.QueryRowContext(ctx, `SELECT mimetype FROM item_attachments LIMIT 1;`).Scan(&mimetype); err != nil {
		t.Fatalf("item_attachments mimetype query error = %v", err)
	}
	if mimetype != "image/png" {
		t.Errorf("item_attachments mimetype = %q; want image/png", mimetype)
	}
	// posts_view surfaces the post with sponsored on the item and a feed/website.
	if got := count(t, s, `SELECT COUNT(*) FROM posts_view WHERE json_extract([post], '$.item.sponsored') = 1 AND json_extract([post], '$.website.html_url') = 'https://hypertexting.com' AND json_extract([post], '$.feed.id') IS NOT NULL;`); got != 1 {
		t.Errorf("posts_view sponsored post = %d; want 1", got)
	}

	// Re-save with the same payload: idempotent (stable identifiers upsert).
	if err := SavePost(ctx, s.Database, post); err != nil {
		t.Fatalf("SavePost() re-save error = %v", err)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM items;`); got != 1 {
		t.Errorf("items after re-save = %d; want 1 (idempotent)", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM items WHERE sponsored = 1;`); got != 1 {
		t.Errorf("sponsored items after re-save = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM item_attachments;`); got != 1 {
		t.Errorf("item_attachments after re-save = %d; want 1 (idempotent)", got)
	}
}

// TestSavePostWritesFollowingVerbatim pins the contract that replaced the old
// preserve-on-conflict logic: SavePost writes follow-state exactly as given on
// the in-memory post (no persistence-layer preserve/recompute). Read-before-
// write — carrying the DB's following into the payload — is the caller's
// responsibility. Together with TestPostSave (verbatim following=false) this
// fixes both directions of the contract.
func TestSavePostWritesFollowingVerbatim(t *testing.T) {
	ctx := context.Background()
	s := newSubscriber(t)
	post := newSponsoredPost(t, "2025-09-09")
	post.Website.Following = true
	post.Feed.Following = true
	if err := SavePost(ctx, s.Database, post); err != nil {
		t.Fatalf("SavePost() error = %v", err)
	}
	if got := count(t, s, `SELECT following FROM websites WHERE id = ?;`, post.Website.Identifier()); got != 1 {
		t.Errorf("website following = %d; want 1 (written verbatim)", got)
	}
	if got := count(t, s, `SELECT following FROM feeds WHERE id = ?;`, post.Feed.Identifier()); got != 1 {
		t.Errorf("feed following = %d; want 1 (written verbatim)", got)
	}
}

// TestPostReadRoundTrip verifies a saved Post reads back via the generic
// ListRecords[*Post] from posts_view with its envelope intact.
func TestPostReadRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newSubscriber(t)
	post := newSponsoredPost(t, "2025-09-09")
	if err := SavePost(ctx, s.Database, post); err != nil {
		t.Fatalf("SavePost() error = %v", err)
	}

	posts, err := database.ListRecords[*hyperfeed.Post](ctx, s.Database)
	if err != nil {
		t.Fatalf("ListRecords[*Post] error = %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("posts = %d; want 1", len(posts))
	}
	got := posts[0]
	if got.Item == nil || !got.Item.Sponsored {
		t.Errorf("post.item.sponsored = %v; want true (item=%+v)", got.Item, got.Item)
	}
	if got.Feed == nil || got.Feed.ID == "" {
		t.Errorf("post.feed missing; want the generated feed")
	}
	if got.Website == nil || got.Website.HTMLURL != "https://hypertexting.com" {
		t.Errorf("post.website = %+v; want sponsor website", got.Website)
	}
}

// TestItemsFeedRequirement documents whether the data model requires a source
// feed for an item. Two distinct answers:
//   - SQL schema: NO. items.feed_id (and website_id) are nullable FKs, so a row
//     with no feed/website is valid and surfaces in posts_view with empty
//     feed/website objects.
//   - Go write path: YES. Item.DatabaseInsertValues derives feed_id/website_id
//     from item.Feed (and item.Feed.Website), so persisting via the ORM
//     (UpsertRow / SavePost) requires a feed — there is no feedless write path.
func TestItemsFeedRequirement(t *testing.T) {
	ctx := context.Background()
	s := newSubscriber(t)

	t.Run("schema allows a feedless item", func(t *testing.T) {
		ts := "2025-09-09T19:00:00Z"
		// Raw insert with NULL feed_id and website_id (foreign_keys is ON).
		if err := s.Database.Exec(ctx,
			`INSERT INTO items (id, url, published_at, added_at, updated_at, feed_at, sponsored) VALUES (?,?,?,?,?,?,0);`,
			"orphan", "https://example.com/orphan", ts, ts, ts, ts); err != nil {
			t.Fatalf("insert feedless item error = %v (schema should allow NULL feed_id/website_id)", err)
		}
		if got := count(t, s, `SELECT COUNT(*) FROM items WHERE id='orphan' AND feed_id IS NULL AND website_id IS NULL;`); got != 1 {
			t.Errorf("feedless item with NULL feed_id/website_id = %d; want 1", got)
		}
		// It still surfaces in posts_view, with empty feed/website objects.
		if got := count(t, s, `SELECT COUNT(*) FROM posts_view WHERE id='orphan' AND json_extract([post],'$.feed.id') IS NULL AND json_extract([post],'$.website.id') IS NULL;`); got != 1 {
			t.Errorf("feedless post (empty feed/website) = %d; want 1", got)
		}
	})

	t.Run("a feedless item still renders its website card", func(t *testing.T) {
		ts := "2025-09-10T19:00:00Z"
		// website_id is read directly from items.website_id (a sibling join in
		// posts_view) — NOT through the feed. So an item with a website but no
		// feed shows its website card; only the feed object is empty.
		if err := s.Database.Exec(ctx,
			`INSERT INTO websites (id, html_url, name, following) VALUES (?,?,?,0);`,
			"w1", "https://sponsor.example", "Sponsor"); err != nil {
			t.Fatalf("insert website error = %v", err)
		}
		if err := s.Database.Exec(ctx,
			`INSERT INTO items (id, website_id, url, published_at, added_at, updated_at, feed_at, sponsored) VALUES (?,?,?,?,?,?,?,1);`,
			"orphan-with-site", "w1", "https://sponsor.example/1", ts, ts, ts, ts); err != nil {
			t.Fatalf("insert feedless-but-website item error = %v", err)
		}
		if got := count(t, s, `SELECT COUNT(*) FROM posts_view WHERE id='orphan-with-site' AND json_extract([post],'$.website.id')='w1' AND json_extract([post],'$.website.html_url')='https://sponsor.example' AND json_extract([post],'$.feed.id') IS NULL AND json_extract([post],'$.item.sponsored')=1;`); got != 1 {
			t.Errorf("feedless item with a website in posts_view = %d; want 1 (website card renders without a feed)", got)
		}
	})

	t.Run("Go ORM write path requires a feed", func(t *testing.T) {
		// item.Feed is nil; DatabaseInsertValues derefs item.Feed(.Website) to
		// derive feed_id/website_id, so the ORM path cannot write a feedless item.
		item := &hyperfeed.Item{ID: "no-feed", URL: mustParse(t, "https://example.com/no-feed")}
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("UpsertRow of an item with no feed did not panic; the Go write path is expected to require a source feed")
			}
		}()
		_ = s.Database.UpsertRow(ctx, nil, item)
	})
}

// TestFeedUnfollow verifies subscriber.Unfollow deletes only that feed's items
// and sets following=0, leaving other feeds' items intact.
func TestFeedUnfollow(t *testing.T) {
	ctx := context.Background()
	s := newSubscriber(t)

	keep := newSponsoredPost(t, "2025-09-09")
	keep.Feed.URL = mustParse(t, "https://keep.example/feed.json")
	keep.Feed.HTMLURL = mustParse(t, "https://keep.example")
	keep.Website = &hyperfeed.Website{Entity: hypergraph.Entity{HTMLURL: "https://keep.example"}, Following: true}
	keep.Feed.Following = true
	keep.Item.URL = mustParse(t, "https://keep.example/post1")
	keep.Item.Sponsored = false
	if err := SavePost(ctx, s.Database, keep); err != nil {
		t.Fatalf("SavePost(keep) error = %v", err)
	}

	drop := newSponsoredPost(t, "2025-09-10")
	drop.Feed.Status.LastStatusCode = 304
	drop.Feed.Status.LastStatusCodes = []int{304, 200}
	if err := SavePost(ctx, s.Database, drop); err != nil {
		t.Fatalf("SavePost(drop) error = %v", err)
	}

	if got := count(t, s, `SELECT COUNT(*) FROM items;`); got != 2 {
		t.Fatalf("items before unfollow = %d; want 2", got)
	}
	if got := count(t, s, `SELECT json_extract(status, '$.last_status_code') FROM feeds WHERE id = ?;`, drop.Feed.Identifier()); got != 304 {
		t.Fatalf("dropped feed status before unfollow = %d; want 304", got)
	}

	if err := Unfollow(ctx, s.Database, drop.Feed); err != nil {
		t.Fatalf("Unfollow() error = %v", err)
	}

	if got := count(t, s, `SELECT COUNT(*) FROM items WHERE feed_id = ?;`, drop.Feed.Identifier()); got != 0 {
		t.Errorf("dropped feed items = %d; want 0", got)
	}
	if got := count(t, s, `SELECT following FROM feeds WHERE id = ?;`, drop.Feed.Identifier()); got != 0 {
		t.Errorf("dropped feed following = %d; want 0", got)
	}
	if got := count(t, s, `SELECT json_extract(status, '$.last_status_code') FROM feeds WHERE id = ?;`, drop.Feed.Identifier()); got != 0 {
		t.Errorf("dropped feed status after unfollow = %d; want 0", got)
	}
	if got := count(t, s, `SELECT json_array_length(status, '$.last_status_codes') FROM feeds WHERE id = ?;`, drop.Feed.Identifier()); got != 0 {
		t.Errorf("dropped feed status history after unfollow = %d; want 0", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM items WHERE feed_id = ?;`, keep.Feed.Identifier()); got != 1 {
		t.Errorf("kept feed items = %d; want 1 (must not be deleted)", got)
	}
}

// TestUpsertFollowingFalsePreservesItems verifies the non-destructive invariant:
// writing following=false through the generic upsert (the refresh/import path)
// does NOT delete the feed's items — only Unfollow does.
func TestUpsertFollowingFalsePreservesItems(t *testing.T) {
	ctx := context.Background()
	s := newSubscriber(t)
	post := newSponsoredPost(t, "2025-09-09")
	post.Website.Following = true
	post.Feed.Following = true
	if err := SavePost(ctx, s.Database, post); err != nil {
		t.Fatalf("SavePost() error = %v", err)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM items WHERE feed_id = ?;`, post.Feed.Identifier()); got != 1 {
		t.Fatalf("items before = %d; want 1", got)
	}

	// Generic upsert with following=false (NOT Feed.Unfollow) — items must remain.
	post.Feed.Following = false
	if err := s.Database.UpsertRow(ctx, nil, post.Feed); err != nil {
		t.Fatalf("UpsertRow() error = %v", err)
	}
	if got := count(t, s, `SELECT following FROM feeds WHERE id = ?;`, post.Feed.Identifier()); got != 0 {
		t.Errorf("feed following = %d; want 0", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM items WHERE feed_id = ?;`, post.Feed.Identifier()); got != 1 {
		t.Errorf("items after following=false upsert = %d; want 1 (only Unfollow deletes)", got)
	}
}
