package transporter

import (
	"testing"
)

// TestRequestValidRejectsBackoff documents the expectation that Request.Valid
// refuses to run while NotBefore is still in the future and returns
// ErrRequestNotReady so callers can treat it as a no-op.
func TestRequestValidRejectsBackoff(t *testing.T) {
	t.Skip("implementation pending: Request.Valid must surface ErrRequestNotReady when guardrails block execution")
}

// TestTransporterFetchNoopWhenNotReady documents the expectation that Fetch
// performs preflight checks and returns the same typed error used by
// Request.Valid when a request must be deferred.
func TestTransporterFetchNoopWhenNotReady(t *testing.T) {
	t.Skip("implementation pending: Fetch must return ErrRequestNotReady when preflight fails")
}

// TestResponseExposesHeaders documents the expectation that the transporter
// surfaces raw headers for the scheduler to interpret instead of translating
// them into scheduling directives.
func TestResponseExposesHeaders(t *testing.T) {
	t.Skip("implementation pending: transporter must populate response headers for scheduler interpretation")
}

func TestTransporterFetchPopulatesResponse(t *testing.T) {
	t.Skip("implementation pending: transporter must populate response fields and capture conditions after fetches")
}

func TestTransporterFetchAppliesOptionHeaders(t *testing.T) {
	t.Skip("implementation pending: transporter must apply Options.Headers and deduplicate against conditional headers")
}

func TestTransporterFetchRespectsBodyLimit(t *testing.T) {
	t.Skip("implementation pending: transporter must enforce Config.MaxBodySize and return ErrResponseTooLarge")
}
