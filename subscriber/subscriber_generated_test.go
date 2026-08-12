package subscriber

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/href"
)

// writeGeneratedFeed writes a minimal single-item JSON Feed file to path (used
// as a refreshable fixture for the file:// refresh tests).
func writeGeneratedFeed(t *testing.T, path, homePage, itemURL string, published time.Time) {
	t.Helper()
	doc := fmt.Sprintf(`{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "Sponsors",
  "home_page_url": %q,
  "items": [ { "id": %q, "url": %q, "title": "Sponsored", "summary": "A sponsored post.", "date_published": %q } ]
}`, homePage, itemURL, itemURL, published.UTC().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func seedFileFeed(t *testing.T, ctx context.Context, s *Subscriber, path string, following bool) *hyperfeed.Feed {
	t.Helper()
	u, err := href.Parse("file://" + path)
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	feed := &hyperfeed.Feed{URL: u, Following: following}
	feed.ID = feed.Identifier()
	if err := s.Database.UpsertRow(ctx, nil, feed); err != nil {
		t.Fatalf("UpsertRow() error = %v", err)
	}
	return feed
}

// TestRefreshFileFeedSkipsScrape verifies that refreshing a file:// (generated)
// feed never triggers the HyperGraph website-metadata scrape: the feed's
// home_page_url points at a test server that must not be hit.
func TestRefreshFileFeedSkipsScrape(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := &Subscriber{}
	if err := s.Init(ctx, &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("<html><head><title>Sponsor</title></head><body></body></html>"))
	}))
	defer server.Close()

	feedPath := filepath.Join(dir, "sponsored.json")
	// home_page_url is the (would-be-scraped) server; published in the past.
	writeGeneratedFeed(t, feedPath, server.URL, server.URL+"/sponsors/2025-09-09", time.Date(2025, 9, 9, 19, 0, 0, 0, time.UTC))

	feed := seedFileFeed(t, ctx, s, feedPath, false)
	if err := s.Refresh(ctx, []*hyperfeed.Feed{feed}, 10, 1, io.Discard); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("home_page_url was fetched %d time(s); want 0 (file:// feeds must skip the scrape)", got)
	}
}

// TestImportFeedsAreFollowed verifies OPML import marks feeds followed
// (following=1) — now that following is a written column, InitFeeds must set it
// explicitly or imports would silently land as unfollowed.
func TestImportFeedsAreFollowed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := &Subscriber{}
	if err := s.Init(ctx, &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	opml := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Subs</title></head><body>
  <outline text="Example" type="rss" xmlUrl="https://example.com/feed.xml" htmlUrl="https://example.com/"/>
</body></opml>`
	opmlPath := filepath.Join(dir, "subs.opml")
	if err := os.WriteFile(opmlPath, []byte(opml), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	if _, err := s.ImportFeeds(ctx, opmlPath, io.Discard); err != nil {
		t.Fatalf("ImportFeeds() error = %v", err)
	}

	var following int
	if err := s.Database.QueryRowContext(ctx, `SELECT following FROM feeds WHERE url = ?;`, "https://example.com/feed.xml").Scan(&following); err != nil {
		t.Fatalf("query following error = %v", err)
	}
	if following != 1 {
		t.Errorf("imported feed following = %d; want 1", following)
	}
}

// TestExportExcludesUnfollowedFeeds verifies OPML export only includes followed
// feeds (following=1), excluding both unfollowed and generated feeds.
func TestExportExcludesUnfollowedFeeds(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := &Subscriber{}
	if err := s.Init(ctx, &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Feeds without a website (website_id null) avoid the FK to websites and are
	// sufficient for OPML export, which keys off the feed URL.
	followedURL, _ := href.Parse("https://followed.example/feed.xml")
	followed := &hyperfeed.Feed{URL: followedURL, Following: true}
	followed.ID = followed.Identifier()
	if err := s.Database.UpsertRow(ctx, nil, followed); err != nil {
		t.Fatalf("UpsertRow(followed) error = %v", err)
	}

	generatedURL, _ := href.Parse("https://sponsors.example/feed.xml")
	generated := &hyperfeed.Feed{URL: generatedURL}
	generated.ID = generated.Identifier()
	if err := s.Database.UpsertRow(ctx, nil, generated); err != nil {
		t.Fatalf("UpsertRow(generated) error = %v", err)
	}
	if err := s.Database.Exec(ctx, `UPDATE feeds SET following = 0 WHERE id = ?;`, generated.ID); err != nil {
		t.Fatalf("unfollow generated error = %v", err)
	}

	opml, err := s.Export(ctx, false, io.Discard)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	out := string(opml)
	if !strings.Contains(out, "followed.example/feed.xml") {
		t.Errorf("Export() missing the followed feed:\n%s", out)
	}
	if strings.Contains(out, "sponsors.example") {
		t.Errorf("Export() leaked an unfollowed/generated feed:\n%s", out)
	}
}

// TestRefreshSkipsUnfollowedFeeds verifies that a no-args Refresh (load-all)
// refreshes followed feeds only and never re-fetches unfollowed (following=0)
// feeds — so a soft-deleted feed's items stay purged.
func TestRefreshSkipsUnfollowedFeeds(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := &Subscriber{}
	if err := s.Init(ctx, &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	followedPath := filepath.Join(dir, "followed.json")
	writeGeneratedFeed(t, followedPath, "https://followed.example", "https://followed.example/post1", time.Date(2025, 9, 9, 19, 0, 0, 0, time.UTC))
	unfollowedPath := filepath.Join(dir, "unfollowed.json")
	writeGeneratedFeed(t, unfollowedPath, "https://unfollowed.example", "https://unfollowed.example/post1", time.Date(2025, 9, 9, 19, 0, 0, 0, time.UTC))

	followed := seedFileFeed(t, ctx, s, followedPath, true)
	unfollowed := seedFileFeed(t, ctx, s, unfollowedPath, false)
	if err := s.Database.Exec(ctx, `UPDATE feeds SET following = 0 WHERE id = ?;`, unfollowed.ID); err != nil {
		t.Fatalf("unfollow error = %v", err)
	}
	_ = followed

	// No-args refresh: should load only following=1 feeds.
	if err := s.Refresh(ctx, nil, 10, 1, io.Discard); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	count := func(url string) int {
		var n int
		if err := s.Database.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE url = ?;`, url).Scan(&n); err != nil {
			t.Fatalf("count(%q) error = %v", url, err)
		}
		return n
	}
	if got := count("https://followed.example/post1"); got != 1 {
		t.Errorf("followed feed items = %d; want 1 (it should have been refreshed)", got)
	}
	if got := count("https://unfollowed.example/post1"); got != 0 {
		t.Errorf("unfollowed feed items = %d; want 0 (it must not be re-fetched)", got)
	}
}

// TestRefreshMarksWebsiteFollowed verifies a followed feed's WEBSITE row ends up
// following=1 after a refresh (the SaveWebsites only-upgrade rule) so it appears
// in the Following view; surfaced through websites_view too.
func TestRefreshMarksWebsiteFollowed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := &Subscriber{}
	if err := s.Init(ctx, &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	path := filepath.Join(dir, "followed.json")
	writeGeneratedFeed(t, path, "https://followed.example", "https://followed.example/post1", time.Date(2025, 9, 9, 19, 0, 0, 0, time.UTC))
	feed := seedFileFeed(t, ctx, s, path, true) // following=true
	if err := s.Refresh(ctx, []*hyperfeed.Feed{feed}, 10, 1, io.Discard); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM websites WHERE following = 1;`); got != 1 {
		t.Errorf("followed feed's website (following=1) = %d; want 1", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM websites_view WHERE json_extract([website], '$.following') = 1;`); got != 1 {
		t.Errorf("websites_view following=true = %d; want 1", got)
	}
}

// TestRefreshPreservesFollowing verifies following round-trips across a normal
// (load-all) refresh of an existing followed feed — the value comes from the DB
// (ResolveFeeds/ListRecords) and is written back, not reset to the default.
func TestRefreshPreservesFollowing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := &Subscriber{}
	if err := s.Init(ctx, &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	path := filepath.Join(dir, "followed.json")
	writeGeneratedFeed(t, path, "https://followed.example", "https://followed.example/post1", time.Date(2025, 9, 9, 19, 0, 0, 0, time.UTC))
	feed := seedFileFeed(t, ctx, s, path, true)
	// No-args refresh: following must come from the DB, not the in-memory struct.
	if err := s.Refresh(ctx, nil, 10, 1, io.Discard); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got := count(t, s, `SELECT following FROM feeds WHERE id = ?;`, feed.ID); got != 1 {
		t.Errorf("feed following after refresh = %d; want 1 (must round-trip, not reset)", got)
	}
}
