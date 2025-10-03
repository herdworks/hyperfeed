//go:build integration

package feeder

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
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
				URL: &url.URL{Scheme: "http", Host: "scripting.com", Path: "/rss.xml"},
			},
			err: nil,
		},
		{
			name: "valid atom feeds",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "feeds.kottke.org", Path: "/main"},
			},
			err: nil,
		},
		{
			name: "valid jsonfeed feed",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "daringfireball.net", Path: "/feeds/json"},
			},
			err: nil,
		},
		{
			name: "rss feed with attachments",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "daringfireball.net", Path: "/thetalkshow/rss"},
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
		var db *database.Database = &database.Database{Path: filepath.Join("testdata", "test.db")}
		// if err := db.Destroy(); err != nil {
		// 	t.Fatalf("destroy test.db: %v", err)
		// }
		t.Run(tc.name, func(t *testing.T) {
			var feeder Feeder = Feeder{Feed: tc.feed, Database: db}
			var err error

			// feeder.Tick()
			if err = feeder.Tick(ctx); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if tc.err != nil && err == nil {
				t.Fatalf("expected error\nwanted: %v\n   got nil", tc.err)
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
			if err = feeder.Database.Open(ctx); err == nil {
				t.Cleanup(func() { feeder.Database.Close() })
			} else {
				t.Fatalf("open test.db: %v", err)
			}
			if err = feeder.SaveFeed(ctx); err != nil {
				t.Fatalf("saving feed: %v", err)
			}
			if err = feeder.SaveItems(ctx); err != nil {
				t.Fatalf("saving items: %v", err)
			}
		})
	}
}
