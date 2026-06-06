//go:build integration

package subscriber

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

func TestSubscriberIntegration(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		err  error
	}{
		{
			name: "Valid OPML file",
			uri:  "testdata/integration/following.opml",
			err:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var subscriber Subscriber = Subscriber{}
			var config Config = Config{DatabasePath: filepath.Join(t.TempDir(), "hyperfeed.db")}
			var ctx context.Context = context.Background()
			var feeds []*hyperfeed.Feed
			var err error

			// Initialize the Subscriber
			if err = subscriber.Init(ctx, &config); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("Unexpected error\n\twanted: %v\n\t   got: %v", tc.err, err)
			}

			// Import the OPML file (persists the subscription list and feeds)
			if feeds, err = subscriber.ImportFeeds(ctx, tc.uri, io.Discard); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			}

			// Refresh all subscriptions (fetch all feeds once)
			if err = subscriber.Refresh(ctx, feeds, 30, 10, io.Discard); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			}
		})
	}
}
