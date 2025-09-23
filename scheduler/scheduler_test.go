package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed-go/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// TestEventReadyRespectsRetryAfter documents the expectation that Event.Ready
// defers fetching when Retry-After or backoff windows are still active.
func TestEventReadyRespectsRetryAfter(t *testing.T) {
	t.Skip("implementation pending: Event.Ready must honor Retry-After and persisted backoff windows")

	event := &Event{
		Feed: &hyperfeed.Feed{},
		Status: &Status{
			FeedID:        "feed-id",
			RateLimit: RateLimit{
				BackoffUntil: time.Now().Add(2 * time.Hour),
				RetryAfter: time.Hour,
			},
		},
		TickAt: time.Now(),
	}
	ready, err := event.Ready()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ready {
		t.Fatalf("expected event to be reported as not ready")
	}
	if !errors.Is(err, transporter.ErrRequestNotReady) {
		t.Fatalf("expected ErrRequestNotReady, got %v", err)
	}
}

// TestSchedulerUpdateValidators documents the expectation that conditional request metadata is saved after successful fetches.
func TestSchedulerUpdateValidators(t *testing.T) {
	t.Skip("implementation pending: scheduler must persist conditional validators after transporter responses")

	var scheduler *Scheduler = &Scheduler{}
	var status *Status = &Status{FeedID: "feed-id"}
	var state *Status
	var req *transporter.Request = &transporter.Request{}
	var resp *transporter.Response = &transporter.Response{}
	var err error

	if state, err = scheduler.Update(context.Background(), status, req, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ETag == "" || state.LastModifiedAt.IsZero() {
		t.Fatalf("expected conditions to be persisted")
	}
}

// TestSchedulerLogBackoff documents the expectation that repeated failures trigger exponential backoff even without explicit Retry-After hints.
func TestSchedulerLogBackoff(t *testing.T) {
	t.Skip("implementation pending: scheduler must compute exponential backoff on repeated failures")

	var scheduler *Scheduler = &Scheduler{Config: &Config{ErrorBackoffStep: time.Minute}}
	var status *Status = &Status{FeedID: "feed-id"}
	var state *Status
	var req *transporter.Request = &transporter.Request{}
	var err error

	if state, err = scheduler.Log(context.Background(), status, req, ErrNotImplemented); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.RateLimit.RetryAfter == 0 {
		t.Fatalf("expected scheduler to apply Retry-After rate limiting")
	}
}
