//go:build integration

package feeder

// ============================================================================
// Integration Tests
//
// These tests make real network requests and are not run by default.
// To run these tests, use:
//
//   go test ./... -tags=integration
// ============================================================================

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

func TestSchedulerTickIntegration(t *testing.T) {
	tests := []struct {
		name string
		feed *hyperfeed.Feed
		err  error
	}{
		{
			name: "valid rss feed",
			feed: &hyperfeed.Feed{
				URL: &href.Href{Scheme: "http", Host: "scripting.com", Path: "/rss.xml"},
			},
			err: nil,
		},
		{
			name: "another valid rss feed",
			feed: &hyperfeed.Feed{
				URL: &href.Href{Scheme: "https", Host: "www.thisiscolossal.com", Path: "/feed/"},
			},
			err: nil,
		},
		{
			name: "valid atom feeds",
			feed: &hyperfeed.Feed{
				URL: &href.Href{Scheme: "https", Host: "feeds.kottke.org", Path: "/main"},
			},
			err: nil,
		},
		{
			name: "valid jsonfeed feed",
			feed: &hyperfeed.Feed{
				URL: &href.Href{Scheme: "https", Host: "daringfireball.net", Path: "/feeds/json"},
			},
			err: nil,
		},
		{
			name: "rss feed with attachments",
			feed: &hyperfeed.Feed{
				URL: &href.Href{Scheme: "https", Host: "daringfireball.net", Path: "/thetalkshow/rss"},
			},
			err: nil,
		},
		{
			name: "nil feed",
			feed: nil,
			err:  ErrFeed,
		},
	}
	for _, tc := range tests {
		var ctx context.Context = context.Background()
		var db *database.Database = &database.Database{Path: filepath.Join("testdata", tc.name, "test.db")}
		// if err := db.Destroy(); err != nil {
		// 	t.Fatalf("destroy test.db: %v", err)
		// }
		t.Run(tc.name, func(t *testing.T) {
			var feeder Feeder = Feeder{Feed: tc.feed, Database: db}
			var err error

			// feeder.Database Open() and Initialize()
			if err = feeder.Database.Open(ctx); err == nil {
				t.Cleanup(func() { feeder.Database.Close() })
			} else {
				t.Fatalf("open test.db: %v", err)
			}
			if err = feeder.Database.Initialize(ctx); err != nil {
				t.Fatalf("initialize test.db: %v", err)
			}

			// feeder.Tick()
			if err = feeder.Tick(ctx); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("unexpected error on tick: %v", err)
			}
			if feeder.Status == nil {
				t.Fatalf("no status on tick")
			}
			if feeder.Status.LastAttemptAt.IsZero() {
				t.Fatalf("feeder.Status.LastAttemptAt not set")
			}

			// feeder.Parse()
			if err = feeder.Parse(); err == nil {
				t.Logf("Feed kind:      %s", feeder.RawFeed.Kind())
				t.Logf("Feed raw items: %d", feeder.RawFeed.ItemCount())
				t.Logf("Feed items:     %d", len(feeder.Items))
			} else {
				t.Fatalf("could not parse feed: %v", err)
			}
			if feeder.RawFeed.ItemCount() != len(feeder.Items) {
				t.Fatalf("item count mismatch: RawFeed.ItemCount()=%d but len(Items)=%d", feeder.RawFeed.ItemCount(), len(feeder.Items))
			}

			// Feeder.SaveFeed() and Feeder.SaveItems()
			if err = feeder.SaveFeed(ctx); err != nil {
				t.Fatalf("saving feed: %v", err)
			}
			if err = feeder.SaveItems(ctx); err != nil {
				t.Fatalf("saving items: %v", err)
			}
		})
	}
}

// func TestFeederHTTPHelpers(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		request *Request
// 		err     error
// 	}{
// 		{
// 			"feeds.allenpike.com/feed/",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "feeds.allenpike.com", Path: "/feed/"}},
// 			nil,
// 		},
// 		{
// 			"www.thisiscolossal.com/feed/",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "www.thisiscolossal.com", Path: "/feed/"}},
// 			nil,
// 		},
// 		{
// 			"furbo.org/feed/json",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "furbo.org", Path: "/feed/json"}},
// 			nil,
// 		},
// 		{
// 			"daringfireball.net/feeds/json",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "daringfireball.net", Path: "/feeds/json"}},
// 			nil,
// 		},
// 		{
// 			"inessential.com/feed.json",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "inessential.com", Path: "/feed.json"}},
// 			nil,
// 		},
// 		{
// 			"feeds.kottke.org/json",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "feeds.kottke.org", Path: "/json"}},
// 			nil,
// 		},
// 		{
// 			"jvns.ca/atom.xml",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "jvns.ca", Path: "/atom.xml"}},
// 			nil,
// 		},
// 		{
// 			"www.manton.org/feed/json",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "www.manton.org", Path: "/feed/json"}},
// 			nil,
// 		},
// 		{
// 			"vincode.io/feed.xml",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "vincode.io", Path: "/feed.xml"}},
// 			nil,
// 		},
// 		{
// 			"mjtsai.com/blog/feed/",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "mjtsai.com", Path: "/blog/feed/"}},
// 			nil,
// 		},
// 		{
// 			"netnewswire.blog/feed.json",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "netnewswire.blog", Path: "/feed.json"}},
// 			nil,
// 		},
// 		{
// 			"onefoottsunami.com/feed/json/",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "onefoottsunami.com", Path: "/feed/json/"}},
// 			nil,
// 		},
// 		{
// 			"scripting.com/rss.xml",
// 			&Request{URL: &url.URL{Scheme: "http", Host: "scripting.com", Path: "/rss.xml"}},
// 			nil,
// 		},
// 		{
// 			"feedpress.me/sixcolors?type=xml",
// 			&Request{URL: &url.URL{Scheme: "https", Host: "feedpress.me", Path: "/sixcolors", RawQuery: "type=xml"}},
// 			nil,
// 		},
// 	}
// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			var response *http.Response
// 			var err error
// 			if response, err = transporter.Fetch(context.TODO(), tc.request); tc.err != nil && err == nil {
// 				t.Fatalf("expected error %v, got %v", tc.err, err)
// 			} else if tc.err != nil && errors.Is(err, tc.err) {
// 				t.Logf("got expected error\n")
// 				return
// 			}
// 			t.Logf("%s %+v", tc.request.Request.Method, tc.request.URL.String())
// 			t.Logf("%+v %s", response.Response.StatusCode, http.StatusText(response.Response.StatusCode))
// 			for k, v := range response.Response.Header {
// 				if isVendoredResponseHeader(k) || isDeprecatedResponseHeader(k) || isExperimentalResponseHeader(k) {
// 					continue // skip vendored-, deprecated-, and experimental- headers
// 				}
// 				values := strings.Join(v, ", ")
// 				t.Logf("%s: %+v", http.CanonicalHeaderKey(k), values)
// 			}
// 			t.Logf("response body contains %d bytes", len(response.Bytes))

// 			href := href.Href(*tc.request.URL)
// 			feed := &hyperfeed.Feed{URL: &href}
// 			t.Logf("detected feed kind: %s", feed.DetectKind())
// 		})
// 	}
// }
