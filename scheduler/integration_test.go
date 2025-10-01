//go:build integration

package scheduler

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
			err:  ErrSchedulerRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var scheduler Scheduler = Scheduler{Feed: tc.feed}
			var err error
			if err = scheduler.Tick(context.Background()); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if tc.err != nil && err == nil {
				t.Fatalf("expected error\nwanted: %v\n   got nil", tc.err)
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if scheduler.Status == nil {
				t.Fatalf("no status on tick")
			}
			if scheduler.Status.LastAttemptAt.IsZero() {
				t.Fatalf("scheduler.Status.LastAttemptAt not set")
			}

			// scheduler.status to json
			// if statusBytes, err = json.MarshalIndent(scheduler.Status, "", "  "); err != nil {
			// 	t.Fatalf("could not marshal status to JSON: %v", err)
			// }
			// t.Logf("scheduler status: %s", string(statusBytes))

			if err = scheduler.Feed.Parse(); err != nil {
				t.Fatalf("could not parse feed: %v", err)
			}
			t.Logf("Feed kind:      %s", scheduler.Feed.RawFeed.Kind())
			t.Logf("Feed raw items: %d", scheduler.Feed.RawFeed.ItemCount())
			t.Logf("Feed items:     %d", len(scheduler.Feed.Items))

			// TODO: write to storage!
		})
	}
}
