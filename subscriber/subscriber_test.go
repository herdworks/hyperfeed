package subscriber

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
)

// TestSubscriberInit tests the Init and Ready methods of the Subscriber.
//
// If Init() is provided with valid configuration, Ready() should return true.
func TestSubscriberInit(t *testing.T) {
	tests := []struct {
		name   string
		uri    *url.URL
		config *Config
		err    error
	}{
		{
			name:   "Valid URI and Config",
			uri:    &url.URL{Path: "testdata/unit/nnw_default_subscriptions.opml"},
			config: &Config{},
			err:    nil,
		},
		{
			name:   "Valid URI with nil Config",
			uri:    &url.URL{Path: "testdata/integration/following.opml"},
			config: nil,
			err:    nil,
		},
		{
			name:   "Nil URI with valid Config",
			uri:    nil,
			config: &Config{},
			err:    nil,
		},
		{
			name:   "Nil URI and Config",
			uri:    nil,
			config: nil,
			err:    nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var subscriber Subscriber = Subscriber{}
			var ctx context.Context = context.Background()
			var err error

			// Initialize subscriber
			if tc.config == nil && tc.uri != nil {
				tc.config = &Config{DatabasePath: filepath.Join(filepath.Dir(tc.uri.Path), "hyperfeed.db")}
			} else if tc.config == nil {
				tc.config = &Config{DatabasePath: "testdata/hyperfeed.db"}
			}
			if err = subscriber.Init(ctx, tc.config); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			}

			// Check readiness
			if ready, err := subscriber.Ready(ctx); ready && tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			}

			// Load subscriptionlist (if provided)
			if tc.uri != nil {
				if _, err = subscriber.LoadSubscriptions(ctx, tc.uri.Path); tc.err != nil && errors.Is(err, tc.err) {
					return // got expected error
				} else if err != nil {
					t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
				}
			}

			// Final check: if an error was expected but none occurred
			if tc.err != nil && err == nil {
				t.Fatalf("Expected error\nwanted: %v\n   got: %v", tc.err, err)
			}
		})
	}
}
