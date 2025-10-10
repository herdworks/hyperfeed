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

			// Open (or create) the database
			if err = db.Open(ctx); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}

			// Load the OPML file
			if err = subscriber.Load(); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}

			// Refresh all subscriptions (fetch all feeds once)
			if err = subscriber.Refresh(ctx); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}
		})
	}
}
