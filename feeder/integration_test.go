//go:build integration

package feeder

import (
	"context"
	"errors"
	"net/url"
	"testing"

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
			name: "valid jsonfeed feed",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "daringfireball.net", Path: "/feeds/json"},
			},
			err: nil,
		},
		{
			name: "valid atom feed",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "feeds.kottke.org", Path: "/main"},
			},
			err: nil,
		},
		{
			name: "nil feed",
			feed: nil,
			err:  ErrFeederRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var feeder Feeder = Feeder{Feed: tc.feed}
			var err error
			if err = feeder.Tick(context.Background()); tc.err != nil && errors.Is(err, tc.err) {
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

			// feeder.status to json
			// if statusBytes, err = json.MarshalIndent(feeder.Status, "", "  "); err != nil {
			// 	t.Fatalf("could not marshal status to JSON: %v", err)
			// }
			// t.Logf("feeder status: %s", string(statusBytes))

			if err = feeder.Feed.Parse(); err != nil {
				t.Fatalf("could not parse feed: %v", err)
			}
			t.Logf("Feed kind:      %s", feeder.Feed.RawFeed.Kind())
			t.Logf("Feed raw items: %d", feeder.Feed.RawFeed.ItemCount())
			t.Logf("Feed items:     %d", len(feeder.Feed.Items))

			// TODO: write to storage!
		})
	}
}
