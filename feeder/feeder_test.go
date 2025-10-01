package feeder

import (
	"testing"
)

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
