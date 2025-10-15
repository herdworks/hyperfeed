package subscriber

import (
	"context"
	"errors"
	"net/url"
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
			uri:    &url.URL{Path: "testdata/unit/nnw_default_susbcriptions.opml"},
			config: &Config{},
			err:    nil,
		},
		{
			name:   "Valid URI, nil Config",
			uri:    &url.URL{Path: "testdata/unit/nnw_default_susbcriptions.opml"},
			config: nil,
			err:    ErrReadiness,
		},
		{
			name:   "Nil URI, valid Config",
			uri:    nil,
			config: &Config{},
			err:    ErrSubscriptionList,
		},
		{
			name:   "Nil URI and Config",
			uri:    nil,
			config: nil,
			err:    ErrSubscriptionList,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var subscriber Subscriber = Subscriber{}
			var ctx context.Context = context.Background()
			var err error

			if err = subscriber.Init(ctx, tc.uri, tc.config); err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil && errors.Is(err, tc.err) {
				return // got expected error
			}

			// Check readiness
			if ready, err := subscriber.Ready(ctx); ready && err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("Unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			} else if err != nil || errors.Is(err, tc.err) {
				return // got expected error
			}
		})
	}
}