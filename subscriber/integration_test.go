//go:build integration

package subscriber

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

func TestSubscriberIntegration(t *testing.T) {
	tests := []struct {
		name string
		uri  *url.URL
		err  error
	}{
		{
			name: "Valid OPML file",
			uri:  &url.URL{Scheme: "file", Path: "testdata/nnw_default_subscriptions.opml"},
			err:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ctx context.Context = context.Background()
			var db *database.Database = &database.Database{Path: filepath.Join("testdata", "test.db")}
			var subscriptionList hyperfeed.SubscriptionList = hyperfeed.SubscriptionList{URI: tc.uri}
			var subscriber Subscriber = Subscriber{SubscriptionList: subscriptionList, Database: db}
			var err error

			// Load the OPML file
			if err = subscriber.Load(); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}

			// Initialize the Subscriber
			if err = subscriber.Init(); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}

			// Check that the Subscriber has the expected number of feeds
			if len(subscriber.Feeds) != len(subscriber.OPML.Body.Outlines) {
				t.Fatalf("Unexpected number of feeds\nwanted: %d\n   got: %d", len(subscriber.OPML.Body.Outlines), len(subscriber.Feeds))
			}

			// Sync all subscriptions (fetch all feeds once)
			if err = subscriber.Sync(ctx); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}
		})
	}
}
