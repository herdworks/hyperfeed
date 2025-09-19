package transporter

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestTransporterPrepareRequestAddsConditionalHeaders documents the expectation
// that stored validators are attached when building requests.
func TestTransporterPrepareRequestAddsConditionalHeaders(t *testing.T) {
	t.Skip("implementation pending: transporter must populate conditional headers before dispatch")

	transporter := &Transporter{}
	plan := &RequestPlan{
		URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/feed"},
		Conditional: ConditionalRequestState{
			ETag:         "\"abc\"",
			LastModified: time.Now(),
		},
	}
	req, err := transporter.PrepareRequest(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("If-None-Match") == "" {
		t.Fatalf("expected If-None-Match header to be set")
	}
}

// TestTransporterExecutePlanHonorsRateLimit documents the expectation that
// requests are denied when still inside a backoff window.
func TestTransporterExecutePlanHonorsRateLimit(t *testing.T) {
	t.Skip("implementation pending: transporter must refuse execution when plan is rate limited")

	transporter := &Transporter{}
	plan := &RequestPlan{
		URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/feed"},
		RateLimit: RateLimitState{
			BackoffUntil: time.Now().Add(time.Hour),
		},
	}
	if _, err := transporter.ExecutePlan(context.Background(), plan); err == nil {
		t.Fatalf("expected rate-limit violation to return an error")
	}
}

// TestTransporterInterpretResponseComputesNextNotBefore documents the
// expectation that cache and rate-limit hints influence scheduling.
func TestTransporterInterpretResponseComputesNextNotBefore(t *testing.T) {
	t.Skip("implementation pending: transporter must translate response hints into persistence state")

	transporter := &Transporter{}
	plan := &RequestPlan{}
	summary := &ResponseSummary{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Retry-After": []string{"120"}},
	}
	update, err := transporter.InterpretResponse(context.Background(), plan, summary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update.NextNotBefore.Sub(time.Now()) < time.Minute {
		t.Fatalf("expected Retry-After hint to push next fetch into the future")
	}
}
