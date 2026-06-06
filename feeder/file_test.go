package feeder

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	reader "github.com/herdworks/hyperfeed-go/reader"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	"github.com/herdworks/hyperfeed-go/types/href"
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

// TestFeederTickFileURL verifies that Feeder.Tick reads and parses a local
// feed file via a file:// URL, and that an unchanged file yields a 304 Not
// Modified (via If-Modified-Since vs mtime) without clobbering the detected
// feed kind.
func TestFeederTickFileURL(t *testing.T) {
	var ctx = context.Background()
	var client *http.Client = reader.Client(reader.Config{Timeout: 10 * time.Second})

	path := filepath.Join(t.TempDir(), "feed.xml")
	if err := os.WriteFile(path, []byte(fileTestRSS), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	feedURL, err := href.Parse("file://" + path)
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	feed := &hyperfeed.Feed{URL: feedURL}
	feeder := &Feeder{Feed: feed}

	// first tick: read and parse the file
	if err := feeder.Tick(ctx, client, time.Now().UTC(), io.Discard); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if feed.Status.LastStatusCode != http.StatusOK {
		t.Fatalf("Tick() status = %d; want %d", feed.Status.LastStatusCode, http.StatusOK)
	}
	if feed.Kind != "rss" {
		t.Errorf("Tick() kind = %q; want %q", feed.Kind, "rss")
	}
	if len(feed.Items) != 2 {
		t.Errorf("Tick() items = %d; want 2", len(feed.Items))
	}
	if feed.Status.LastModified.IsZero() {
		t.Errorf("Tick() expected Last-Modified from file mtime; got zero time")
	}
	if feed.Status.LastSuccessAt.IsZero() {
		t.Errorf("Tick() expected LastSuccessAt to be set; got zero time")
	}
	if feed.HTMLURL.String() != "https://example.com/" {
		t.Errorf("Tick() html url = %q; want %q", feed.HTMLURL.String(), "https://example.com/")
	}

	// second tick: unchanged file should yield 304 Not Modified and preserve kind
	if err := feeder.Tick(ctx, client, time.Now().UTC(), io.Discard); err != nil {
		t.Fatalf("Tick() second error = %v", err)
	}
	if feed.Status.LastStatusCode != http.StatusNotModified {
		t.Fatalf("Tick() second status = %d; want %d", feed.Status.LastStatusCode, http.StatusNotModified)
	}
	if feed.Kind != "rss" {
		t.Errorf("Tick() second kind = %q; want %q (304 must not clobber kind)", feed.Kind, "rss")
	}
}

// TestFeederTickFileURLMissing verifies that a missing local feed file
// surfaces as an HTTP 404 error response.
func TestFeederTickFileURLMissing(t *testing.T) {
	var ctx = context.Background()
	var client *http.Client = reader.Client(reader.Config{Timeout: 10 * time.Second})

	feedURL, err := href.Parse("file://" + filepath.Join(t.TempDir(), "missing.xml"))
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	feeder := &Feeder{Feed: &hyperfeed.Feed{URL: feedURL}}
	if err := feeder.Tick(ctx, client, time.Now().UTC(), io.Discard); err == nil {
		t.Fatalf("Tick() expected error for missing file; got nil")
	}
	if feeder.Feed.Status.LastStatusCode != http.StatusNotFound {
		t.Errorf("Tick() status = %d; want %d", feeder.Feed.Status.LastStatusCode, http.StatusNotFound)
	}
}

// TestFeederTickRejectsRedirectToFile verifies that a remote feed server cannot
// redirect a refresh into the file transport to read a local file.
func TestFeederTickRejectsRedirectToFile(t *testing.T) {
	var ctx = context.Background()
	var client *http.Client = reader.Client(reader.Config{Timeout: 10 * time.Second})

	secret := filepath.Join(t.TempDir(), "feed.xml")
	if err := os.WriteFile(secret, []byte(fileTestRSS), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file://"+secret, http.StatusFound)
	}))
	defer server.Close()

	feedURL, err := href.Parse(server.URL + "/feed.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	feeder := &Feeder{Feed: &hyperfeed.Feed{URL: feedURL}}
	if err := feeder.Tick(ctx, client, time.Now().UTC(), io.Discard); err == nil {
		t.Fatalf("Tick() followed cross-scheme redirect to a local file; want error")
	}
	if len(feeder.Feed.Items) != 0 {
		t.Errorf("Tick() loaded %d items from a redirected local file; want 0", len(feeder.Feed.Items))
	}
}
