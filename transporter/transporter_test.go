package transporter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// TestRequestValidRejectsBackoff documents the expectation that Request.Valid
// refuses to run while NotBefore is still in the future and returns
// ErrRequestNotReady so callers can treat it as a no-op.
func TestRequestValidRejectsBackoff(t *testing.T) {
	t.Skip("implementation pending: Request.Valid must surface ErrRequestNotReady when guardrails block execution")

	req := &Request{
		Request: &http.Request{},
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
		Request: &http.Request{},
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

func TestTransporterFetchPopulatesResponse(t *testing.T) {
	t.Parallel()

	body := []byte("<rss><channel></channel></rss>")
	etag := `"etag-value"`
	lastModified := time.Date(2024, time.January, 12, 15, 4, 5, 0, time.UTC)
	digestSum := sha256.Sum256(body)
	digestValue := base64.StdEncoding.EncodeToString(digestSum[:])

	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("expected user agent header to be set")
		}
		if got := r.Header.Get("Accept"); got != defaultAcceptHeader {
			t.Fatalf("unexpected Accept header: %q", got)
		}

		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
		w.Header().Set("Digest", "sha-256="+digestValue)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error constructing request: %v", err)
	}

	transporter := &Transporter{}
	response, err := transporter.Fetch(context.Background(), &Request{Request: req})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response == nil {
		t.Fatalf("expected response, got nil")
	}
	defer response.Response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}
	if string(response.Body) != string(body) {
		t.Fatalf("unexpected body: %s", string(response.Body))
	}
	if response.BodyDigest != digestValue {
		t.Fatalf("unexpected body digest: %s", response.BodyDigest)
	}
	if response.BodyDigestAlgorithm != "sha-256" {
		t.Fatalf("unexpected body digest algorithm: %s", response.BodyDigestAlgorithm)
	}
	if response.Headers.Get("Etag") != etag {
		t.Fatalf("expected response headers to retain ETag")
	}
	if response.Duration < 0 {
		t.Fatalf("expected non-negative duration, got %s", response.Duration)
	}
	if response.ReceivedAt.IsZero() {
		t.Fatalf("expected received timestamp to be recorded")
	}

	var hasETag, hasLastModified, hasDigest, hasLength bool
	for _, condition := range response.Conditions {
		switch condition.Kind {
		case ConditionIfNoneMatchStrong, ConditionIfNoneMatchWeak:
			if condition.Value == etag {
				hasETag = true
			}
		case ConditionIfModifiedSince:
			if condition.Value == lastModified.Format(http.TimeFormat) {
				hasLastModified = true
			}
		case ConditionDigest:
			if condition.Value == digestValue && condition.Algorithm == "sha-256" {
				hasDigest = true
			}
		case ConditionContentLength:
			if condition.Value == strconv.Itoa(len(body)) {
				hasLength = true
			}
		}
	}
	if !hasETag {
		t.Fatalf("expected ETag condition to be captured")
	}
	if !hasLastModified {
		t.Fatalf("expected Last-Modified condition to be captured")
	}
	if !hasDigest {
		t.Fatalf("expected Digest condition to be captured")
	}
	if !hasLength {
		t.Fatalf("expected Content-Length condition to be captured")
	}

	data, err := io.ReadAll(response.Response.Body)
	if err != nil {
		t.Fatalf("unexpected error reading response body: %v", err)
	}
	if string(data) != string(body) {
		t.Fatalf("unexpected response body contents: %s", string(data))
	}

	if got := receivedHeaders.Get("User-Agent"); got != UserAgent {
		t.Fatalf("unexpected user agent header observed by server: %q", got)
	}
}

func TestTransporterFetchAppliesOptionHeaders(t *testing.T) {
	t.Parallel()

	var receivedRequest *http.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r.Clone(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error constructing request: %v", err)
	}
	req.Header.Set("Accept", "text/plain")

	condition := &Condition{Kind: ConditionIfNoneMatchStrong, Value: `"etag"`}

	transporter := &Transporter{}
	response, err := transporter.Fetch(context.Background(), &Request{
		Request:    req,
		Conditions: []*Condition{condition},
		Options: Options{Headers: map[string]string{
			"User-Agent": "custom-agent/1.0",
			"X-Debug":    "enabled",
			"Host":       "example.test",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response == nil {
		t.Fatalf("expected response, got nil")
	}
	defer response.Response.Body.Close()

	if receivedRequest == nil {
		t.Fatalf("expected server to receive request")
	}
	if got := receivedRequest.Header.Get("User-Agent"); got != "custom-agent/1.0" {
		t.Fatalf("unexpected user agent header: %q", got)
	}
	if got := receivedRequest.Header.Get("If-None-Match"); got != `"etag"` {
		t.Fatalf("expected conditional header to be forwarded, got %q", got)
	}
	if got := receivedRequest.Header.Get("X-Debug"); got != "enabled" {
		t.Fatalf("expected custom header to be applied, got %q", got)
	}
	if got := receivedRequest.Header.Get("Accept"); got != "text/plain" {
		t.Fatalf("expected explicit Accept header to be preserved, got %q", got)
	}
	if receivedRequest.Host != "example.test" {
		t.Fatalf("expected host override to be applied, got %q", receivedRequest.Host)
	}
}

func TestTransporterFetchRespectsBodyLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error constructing request: %v", err)
	}

	transporter := &Transporter{Config: &Config{Timeout: time.Second, MaxBodySize: 4}}
	resp, err := transporter.Fetch(context.Background(), &Request{Request: req})
	if err == nil {
		if resp != nil {
			resp.Response.Body.Close()
		}
		t.Fatalf("expected error when response exceeds maximum size")
	}
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge, got %v", err)
	}
}
