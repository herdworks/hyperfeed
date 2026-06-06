package subscriber

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	reader "github.com/herdworks/hyperfeed-go/reader"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

const fileTestRSS = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
	<channel>
		<title>Generated Feed</title>
		<link>https://example.com/</link>
		<description>A locally generated feed</description>
		<item>
			<title>First</title>
			<link>https://example.com/posts/first/</link>
			<guid>first</guid>
			<pubDate>Tue, 09 Sep 2025 12:00:00 +0000</pubDate>
			<description>First post</description>
		</item>
		<item>
			<title>Second</title>
			<link>https://example.com/posts/second/</link>
			<guid>second</guid>
			<pubDate>Wed, 10 Sep 2025 12:00:00 +0000</pubDate>
			<description>Second post</description>
		</item>
	</channel>
</rss>`

// TestSubscriberFileFeedEndToEnd verifies the supported generated-feed pipeline:
// a local feed file enters the system via DIRECT DATABASE ACCESS (not OPML
// import, which is http/https only), and Refresh reads + parses + persists it,
// survives the database round-trip with its file:// URL intact, and yields a
// 304 Not Modified on a subsequent refresh of the unchanged file.
func TestSubscriberFileFeedEndToEnd(t *testing.T) {
	var ctx = context.Background()
	var dir = t.TempDir()
	var subscriber *Subscriber = &Subscriber{}
	var config *Config = &Config{DatabasePath: filepath.Join(dir, "hyperfeed.db")}

	// write the generated feed file
	feedPath := filepath.Join(dir, "feed.xml")
	if err := os.WriteFile(feedPath, []byte(fileTestRSS), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	feedURL := "file://" + feedPath

	if err := subscriber.Init(ctx, config); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// insert the local feed directly into the database (the only supported way
	// to introduce a file:// feed; mirrors the CLI `add` / generated-feeds path)
	u, err := href.Parse(feedURL)
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	seed := &hyperfeed.Feed{URL: u}
	seed.ID = seed.Identifier()
	// mark entity metadata fresh so Refresh skips the hypergraph scrape
	// (keeps the test hermetic; no outbound network)
	seed.Status.LastEntityUpdateAt = time.Now().UTC()
	if err := subscriber.Database.UpsertRow(ctx, nil, seed); err != nil {
		t.Fatalf("UpsertRow() error = %v", err)
	}
	feeds := []*hyperfeed.Feed{seed}

	// refresh: read the file, parse, persist feed + items
	if err := subscriber.Refresh(ctx, feeds, 10, 1, io.Discard); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if feeds[0].Status.LastStatusCode != http.StatusOK {
		t.Fatalf("Refresh() status = %d; want %d", feeds[0].Status.LastStatusCode, http.StatusOK)
	}
	if feeds[0].Kind != "rss" {
		t.Errorf("Refresh() kind = %q; want %q", feeds[0].Kind, "rss")
	}

	// verify items persisted
	var itemCount int
	if err := subscriber.Database.QueryRowContext(ctx, `SELECT COUNT(*) FROM items;`).Scan(&itemCount); err != nil {
		t.Fatalf("counting items error = %v", err)
	}
	if itemCount != 2 {
		t.Errorf("items count = %d; want 2", itemCount)
	}

	// verify the file:// URL survives the database round-trip
	restored := &hyperfeed.Feed{URL: feeds[0].URL}
	if err := subscriber.Database.ReadRow(ctx, restored); err != nil {
		t.Fatalf("ReadRow() error = %v", err)
	}
	if restored.URL.String() != feedURL {
		t.Errorf("ReadRow() url = %q; want %q", restored.URL.String(), feedURL)
	}
	if !strings.HasPrefix(restored.URL.String(), "file://") {
		t.Errorf("ReadRow() url = %q; want file:// scheme preserved", restored.URL.String())
	}
	if restored.Kind != "rss" {
		t.Errorf("ReadRow() kind = %q; want %q", restored.Kind, "rss")
	}

	// backdate the last success so the interval gate allows another refresh,
	// then verify the unchanged file yields a 304 and preserves the kind
	for _, feed := range feeds {
		feed.Status.LastSuccessAt = feed.Status.LastSuccessAt.Add(-10 * time.Minute)
		feed.Status.LastAttemptAt = feed.Status.LastAttemptAt.Add(-10 * time.Minute)
	}
	if err := subscriber.SaveFeeds(ctx, io.Discard, feeds); err != nil {
		t.Fatalf("SaveFeeds() backdate error = %v", err)
	}
	if err := subscriber.Refresh(ctx, feeds, 10, 1, io.Discard); err != nil {
		t.Fatalf("Refresh() second error = %v", err)
	}
	if feeds[0].Status.LastStatusCode != http.StatusNotModified {
		t.Fatalf("Refresh() second status = %d; want %d", feeds[0].Status.LastStatusCode, http.StatusNotModified)
	}
	if feeds[0].Kind != "rss" {
		t.Errorf("Refresh() second kind = %q; want %q (304 must not clobber kind)", feeds[0].Kind, "rss")
	}
}

// TestLoadSubscriptionListHTTP verifies that OPML subscription lists can be
// read from remote http(s) URLs (scheme-dispatched via the reader package).
func TestLoadSubscriptionListHTTP(t *testing.T) {
	var ctx = context.Background()
	opmlBytes, err := os.ReadFile("testdata/unit/nnw_default_subscriptions.opml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-opml+xml")
		_, _ = w.Write(opmlBytes)
	}))
	defer server.Close()

	subscriptions, err := LoadSubscriptionList(ctx, server.URL+"/subscriptions.opml")
	if err != nil {
		t.Fatalf("LoadSubscriptionList() error = %v", err)
	}
	if len(subscriptions.OPML.Body.Outlines) == 0 {
		t.Fatalf("LoadSubscriptionList() outlines = 0; want > 0")
	}
	if !strings.HasPrefix(subscriptions.URI.String(), "http://") {
		t.Errorf("LoadSubscriptionList() uri = %q; want http:// scheme preserved", subscriptions.URI.String())
	}
}

// TestLoadSubscriptionListLocalPath verifies that bare local paths still work
// (normalized to file:// internally).
func TestLoadSubscriptionListLocalPath(t *testing.T) {
	var ctx = context.Background()
	const path = "testdata/unit/nnw_default_subscriptions.opml"
	subscriptions, err := LoadSubscriptionList(ctx, path)
	if err != nil {
		t.Fatalf("LoadSubscriptionList() error = %v", err)
	}
	if len(subscriptions.OPML.Body.Outlines) == 0 {
		t.Fatalf("LoadSubscriptionList() outlines = 0; want > 0")
	}
	// The stored URI is the normalized form: bare local paths become absolute
	// file:// URLs (see reader.Normalize).
	want, err := reader.Normalize(path)
	if err != nil {
		t.Fatalf("reader.Normalize() error = %v", err)
	}
	if subscriptions.URI.String() != want.String() {
		t.Errorf("LoadSubscriptionList() uri = %q; want %q", subscriptions.URI.String(), want.String())
	}
}

// TestInitFeedsRejectsNonWebFeeds verifies that OPML import only ever yields
// http/https feeds: a non-web (file://) outline is skipped regardless of where
// the OPML itself was read from. Local feeds must enter only via direct
// database access, never via a subscription list.
func TestInitFeedsRejectsNonWebFeeds(t *testing.T) {
	const opmlDoc = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
	<head><title>Mixed</title></head>
	<body>
		<outline text="Local" type="rss" xmlUrl="file:///etc/passwd" htmlUrl="https://example.com/"/>
		<outline text="Remote" type="rss" xmlUrl="https://example.com/feed.xml" htmlUrl="https://example.com/"/>
	</body>
</opml>`

	// Exercise both a remote OPML source and a local OPML file: the file://
	// feed must be rejected in both cases.
	t.Run("remote source", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(opmlDoc))
		}))
		defer server.Close()
		assertOnlyWebFeed(t, server.URL+"/subscriptions.opml")
	})
	t.Run("local source", func(t *testing.T) {
		opmlPath := filepath.Join(t.TempDir(), "subscriptions.opml")
		if err := os.WriteFile(opmlPath, []byte(opmlDoc), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
		assertOnlyWebFeed(t, opmlPath)
	})
}

func assertOnlyWebFeed(t *testing.T, opmlPath string) {
	t.Helper()
	subscriptions, err := LoadSubscriptionList(context.Background(), opmlPath)
	if err != nil {
		t.Fatalf("LoadSubscriptionList() error = %v", err)
	}
	feeds, err := subscriptions.InitFeeds(io.Discard)
	if err != nil {
		t.Fatalf("InitFeeds() error = %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("InitFeeds() feeds = %d; want 1 (file:// outline must be skipped)", len(feeds))
	}
	if got := feeds[0].URL.URL().Scheme; got != "https" {
		t.Errorf("InitFeeds() surviving feed scheme = %q; want https", got)
	}
}
