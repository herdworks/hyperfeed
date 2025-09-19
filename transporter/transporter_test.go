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
		NotBefore:   time.Now().Add(time.Hour),
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
		NotBefore:   time.Now().Add(time.Hour),
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

// TestResponseResolveComputesNextNotBefore documents the expectation that
// Response.Resolve translates Retry-After and cache headers into persistence
// updates for the scheduler.
func TestResponseResolveComputesNextNotBefore(t *testing.T) {
	t.Skip("implementation pending: Response.Resolve must compute PersistenceUpdate hints")

	req := &Request{HTTPRequest: &http.Request{}}
	resp := &Response{
		Headers:    http.Header{"Retry-After": []string{"120"}},
		StatusCode: http.StatusOK,
	}
	update, err := resp.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update.NextNotBefore.Before(time.Now().Add(2 * time.Minute)) {
		t.Fatalf("expected Retry-After to push next fetch into the future")
	}
}
