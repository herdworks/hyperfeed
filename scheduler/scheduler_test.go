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
			NextNotBefore: time.Now().Add(time.Hour),
			RateLimit: transporter.RateLimitState{
				BackoffUntil: time.Now().Add(2 * time.Hour),
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

// TestSchedulerApplyResultPersistsValidators documents the expectation that
// conditional request metadata is saved after successful fetches.
func TestSchedulerApplyResultPersistsValidators(t *testing.T) {
	t.Skip("implementation pending: scheduler must persist conditional validators after transporter responses")

	scheduler := &Scheduler{}
	status := &Status{FeedID: "feed-id"}
	req := &transporter.Request{}
	resp := &transporter.Response{}
	update := &transporter.PersistenceUpdate{}

	newState, err := scheduler.ApplyResult(context.Background(), status, req, resp, update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newState.Conditional.ETag == "" {
		t.Fatalf("expected conditional validators to be persisted")
	}
}

// TestSchedulerRecordFailureBackoff documents the expectation that repeated
// failures trigger exponential backoff even without explicit Retry-After hints.
func TestSchedulerRecordFailureBackoff(t *testing.T) {
	t.Skip("implementation pending: scheduler must compute exponential backoff on repeated failures")

	scheduler := &Scheduler{Config: &Config{ErrorBackoffStep: time.Minute}}
	status := &Status{FeedID: "feed-id"}
	req := &transporter.Request{}

	newState, err := scheduler.RecordFailure(context.Background(), status, req, ErrNotImplemented)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newState.NextNotBefore.Before(time.Now().Add(time.Minute)) {
		t.Fatalf("expected failure backoff to push next fetch into the future")
	}
}
