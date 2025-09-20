package transporter

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestRequestValidRejectsBackoff documents the expectation that Request.Valid
// refuses to run while NotBefore is still in the future and returns
// ErrRequestNotReady so callers can treat it as a no-op.
func TestRequestValidRejectsBackoff(t *testing.T) {
	t.Skip("implementation pending: Request.Valid must surface ErrRequestNotReady when guardrails block execution")

	req := &Request{
		HTTPRequest: &http.Request{},
		Options: Options{
			NotBefore: time.Now().Add(time.Hour),
		},
	}
	ready, err := req.Valid()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ready {
		t.Fatalf("expected request to be reported as not ready")
	}
	if !errors.Is(err, ErrRequestNotReady) {
		t.Fatalf("expected ErrRequestNotReady, got %v", err)
	}
}

// TestTransporterFetchNoopWhenNotReady documents the expectation that Fetch
// performs preflight checks and returns the same typed error used by
// Request.Valid when a request must be deferred.
func TestTransporterFetchNoopWhenNotReady(t *testing.T) {
	t.Skip("implementation pending: Fetch must return ErrRequestNotReady when preflight fails")

	transporter := &Transporter{}
	req := &Request{
		HTTPRequest: &http.Request{},
		Options: Options{
			NotBefore: time.Now().Add(time.Hour),
		},
	}
	resp, err := transporter.Fetch(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if resp != nil {
		t.Fatalf("expected response to be nil when fetch is deferred")
	}
	if !errors.Is(err, ErrRequestNotReady) {
		t.Fatalf("expected ErrRequestNotReady, got %v", err)
	}
}

// TestResponseExposesHeaders documents the expectation that the transporter
// surfaces raw headers for the scheduler to interpret instead of translating
// them into scheduling directives.
func TestResponseExposesHeaders(t *testing.T) {
	t.Skip("implementation pending: transporter must populate response headers for scheduler interpretation")

	resp := &Response{
		Headers:    http.Header{"Retry-After": []string{"120"}},
		StatusCode: http.StatusOK,
	}
	if resp.Headers.Get("Retry-After") == "" {
		t.Fatalf("expected transporter response to retain Retry-After header")
	}
}
