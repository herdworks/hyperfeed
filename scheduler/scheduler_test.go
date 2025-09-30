package scheduler

import (
	"context"
	"errors"
	"net/url"
	"testing"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

func TestSchedulerTick(t *testing.T) {
	tests := []struct {
		name string
		feed *hyperfeed.Feed
		err  error
	}{
		{
			name: "valid feed",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "daringfireball.net", Path: "/feeds/json"},
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
			if err = scheduler.Tick(context.Background()); tc.err != nil && err == nil {
				t.Fatalf("expected error\nwanted: %v\n got nil", tc.err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("unexpected error\nwanted: %v\n   got: %v", tc.err, err)
			}
			if scheduler.Status == nil {
				t.Fatalf("no status on tick")
			}
			if scheduler.Status.LastAttemptAt.IsZero() {
				t.Fatalf("scheduler.Status.LastAttemptAt not set")
			}
		})
	}
}

// TestEventReadyRespectsRetryAfter documents the expectation that Event.Ready
// defers fetching when Retry-After or backoff windows are still active.
func TestEventReadyRespectsRetryAfter(t *testing.T) {
	t.Skip("TODO: Event.Ready must honor Retry-After and persisted backoff windows")
}

// TestSchedulerUpdateValidators documents the expectation that conditional request metadata is saved after successful fetches.
func TestSchedulerUpdateValidators(t *testing.T) {
	t.Skip("TODO: scheduler must persist conditional validators after transporter responses")
}

// TestSchedulerLogBackoff documents the expectation that repeated failures trigger exponential backoff even without explicit Retry-After hints.
func TestSchedulerLogBackoff(t *testing.T) {
	t.Skip("TODO: scheduler must compute exponential backoff on repeated failures")
}
