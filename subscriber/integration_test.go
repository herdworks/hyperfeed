//go:build integration

package subscriber

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestSubscriberIntegration(t *testing.T) {
	tests := []struct {
		name string
		uri  *url.URL
		err  error
	}{
		{
			name: "Valid OPML file",
			uri:  &url.URL{Path: "testdata/integration/following.opml"},
			err:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var subscriber Subscriber = Subscriber{}
			var ctx context.Context = context.Background()
			var err error

			// Initialize the Subscriber
			if err = subscriber.Init(ctx, tc.uri, nil); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}

			// Load the OPML file
			if err = subscriber.Load(ctx); err != nil && !errors.Is(err, tc.err) {
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
