package hyperfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestRecommendedFeedsDiscoversFeedsForRecommendations(t *testing.T) {
	one := recommendedSiteServer(t, "/feed.xml")
	two := recommendedSiteServer(t, "/atom.xml")
	source := recommendationsServer(t, []string{one.URL + "/", two.URL + "/", one.URL + "/", "ftp://skip.example/"})

	feeds, err := RecommendedFeeds(context.Background(), source.URL, 2, 2, false, io.Discard)
	if err != nil {
		t.Fatalf("RecommendedFeeds() error = %v", err)
	}
	got := feedURLs(feeds)
	want := []string{one.URL + "/feed.xml", two.URL + "/atom.xml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("feed URLs = %#v; want %#v", got, want)
	}
	for _, feed := range feeds {
		if feed.Following {
			t.Fatalf("feed %q following = true; want false", feed.URL.String())
		}
		if feed.Website.HTMLURL == "" {
			t.Fatalf("feed %q missing website HTMLURL", feed.URL.String())
		}
	}
}

func TestRecommendedFeedsReturnsEmptyJSONArrayWhenUnavailable(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(source.Close)

	feeds, err := RecommendedFeeds(context.Background(), source.URL, 2, 2, false, io.Discard)
	if err != nil {
		t.Fatalf("RecommendedFeeds() error = %v", err)
	}
	if feeds == nil {
		t.Fatal("RecommendedFeeds() returned nil; want empty slice")
	}
	if len(feeds) != 0 {
		t.Fatalf("RecommendedFeeds() returned %d feeds; want 0", len(feeds))
	}
	bytes, err := json.Marshal(feeds)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(bytes) != "[]" {
		t.Fatalf("json.Marshal() = %s; want []", bytes)
	}
}

func TestRecommendationsOPMLRendersSubscriptionList(t *testing.T) {
	one := recommendedSiteServer(t, "/feed.xml")
	source := recommendationsServer(t, []string{one.URL + "/"})

	opml, err := RecommendationsOPML(context.Background(), source.URL, 2, 1, false, io.Discard)
	if err != nil {
		t.Fatalf("RecommendationsOPML() error = %v", err)
	}
	out := string(opml)
	if !strings.Contains(out, `xmlUrl="`+one.URL+`/feed.xml"`) {
		t.Fatalf("OPML missing feed xmlUrl: %s", out)
	}
	if !strings.Contains(out, `htmlUrl="`+one.URL+`/"`) {
		t.Fatalf("OPML missing recommendation htmlUrl: %s", out)
	}
}

func recommendedSiteServer(t *testing.T, feedPath string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><head><link rel="alternate" type="application/rss+xml" href="%s"></head></html>`, feedPath)
		case feedPath:
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>Recommended</title></channel></rss>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func recommendationsServer(t *testing.T, urls []string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/recommendations.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recommendationsJSON(urls)))
	}))
	t.Cleanup(server.Close)
	return server
}

func recommendationsJSON(urls []string) string {
	entries := make([]string, 0, len(urls))
	for _, url := range urls {
		entries = append(entries, fmt.Sprintf(`{"url":%q}`, url))
	}
	return "[" + strings.Join(entries, ",") + "]"
}

func feedURLs(feeds []*Feed) []string {
	urls := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		urls = append(urls, feed.URL.String())
	}
	return urls
}
