package feeder

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	"github.com/herdworks/hyperfeed-go/types/href"
)

type closeTrackingBody struct {
	*strings.Reader
	closed bool
}

func (body *closeTrackingBody) Close() error {
	body.closed = true
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestTickClosesResponseBody(t *testing.T) {
	body := &closeTrackingBody{Reader: strings.NewReader(fileTestRSS)}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, nil
	})}
	feedURL, err := href.Parse("https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	feeder := &Feeder{Feed: &hyperfeed.Feed{URL: feedURL}}

	if err := feeder.Tick(context.Background(), client, time.Now().UTC(), io.Discard); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if !body.closed {
		t.Fatalf("Tick() did not close response body")
	}
}

func TestTickClosesResponseBodyOnHTTPError(t *testing.T) {
	body := &closeTrackingBody{Reader: strings.NewReader("server error")}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, nil
	})}
	feedURL, err := href.Parse("https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	feeder := &Feeder{Feed: &hyperfeed.Feed{URL: feedURL}}

	if err := feeder.Tick(context.Background(), client, time.Now().UTC(), io.Discard); err == nil {
		t.Fatalf("Tick() expected HTTP error; got nil")
	}
	if !body.closed {
		t.Fatalf("Tick() did not close error response body")
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
