package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/herdworks/hyperfeed-go/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// TestSchedulerNextDecisionRespectsRetryAfter documents the expectation that a
// Retry-After header moves the next decision window into the future.
func TestSchedulerNextDecisionRespectsRetryAfter(t *testing.T) {
	t.Skip("implementation pending: scheduler must honor Retry-After and persisted backoff windows")

	scheduler := &Scheduler{}
	feed := &hyperfeed.Feed{}
	state := &State{
		FeedID:        "feed-id",
		NextNotBefore: time.Now().Add(time.Hour),
		RateLimit: transporter.RateLimitState{
			BackoffUntil: time.Now().Add(2 * time.Hour),
		},
	}
	_ = state
	decision, _, err := scheduler.NextDecision(context.Background(), feed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionDefer {
		t.Fatalf("expected ActionDefer, got %v", decision.Action)
	}
}

// TestSchedulerApplyResultPersistsValidators documents the expectation that
// conditional request metadata is saved after successful fetches.
func TestSchedulerApplyResultPersistsValidators(t *testing.T) {
	t.Skip("implementation pending: scheduler must persist conditional validators after transporter responses")

	scheduler := &Scheduler{}
	state := &State{FeedID: "feed-id"}
	plan := &transporter.RequestPlan{}
	summary := &transporter.ResponseSummary{}
	update := &transporter.PersistenceUpdate{}

	newState, err := scheduler.ApplyResult(context.Background(), state, plan, summary, update)
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

	scheduler := &Scheduler{Options: Options{ErrorBackoffStep: time.Minute}}
	state := &State{FeedID: "feed-id"}
	plan := &transporter.RequestPlan{}

	newState, err := scheduler.RecordFailure(context.Background(), state, plan, ErrNotImplemented)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newState.NextNotBefore.Before(time.Now().Add(time.Minute)) {
		t.Fatalf("expected failure backoff to push next fetch into the future")
	}
}
