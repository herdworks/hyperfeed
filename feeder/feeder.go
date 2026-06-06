package feeder

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/http2"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
	version "github.com/herdworks/hyperfeed-go/types/version"
)

// TYPES
// ============================================================================

// Feeder manages a Feed.
type Feeder struct {
	*hyperfeed.Feed
}

// VARIABLES
// ============================================================================

// errors
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrReadiness      = errors.New("feeder not ready")
	ErrPaused         = errors.New("feed paused")
	ErrTooSoonJunior  = errors.New("too soon, junior")
	ErrUnhealthy      = errors.New("feed is unhealthy")
	ErrThrottled      = errors.New("throttled to avoid rate limiting")
	ErrRateLimited    = errors.New("rate limited")
	ErrFeed           = errors.New("invalid feed")
	ErrFeedParser     = errors.New("feed parser error")
	ErrRequest        = errors.New("request error")
	ErrFetch          = errors.New("fetch error")
	ErrResponse       = errors.New("response error")
	ErrDNS            *net.DNSError
	ErrTLSHostName    x509.HostnameError
	ErrTLSInvalid     x509.CertificateInvalidError
	ErrTLSUnknownCA   x509.UnknownAuthorityError

	DefaultAcceptHeaders = []string{
		"application/rss+xml",
		"application/atom+xml;q=0.9",
		"application/feed+json;q=0.8",
		"application/xml;q=0.7",
		"text/xml;q=0.5",
		"application/json;q=0.4",
		"*/*;q=0.1",
	}
)

// METHODS
// =============================================================================

// Tick handles a feed scheduling event.
// It performs readiness checks, builds and sends the request, applies the response.
func (feeder *Feeder) Tick(ctx context.Context, client *http.Client, at time.Time, w io.Writer) error {
	var ready bool
	var request *http.Request
	var response *http.Response
	var body []byte
	var err error
	if w == nil {
		w = os.Stdout
	}
	feeder.Feed.Status.UpdatedAt = at
	if ready, err = feeder.Ready(); !ready {
		feeder.Log(nil, errors.Join(ErrReadiness, err), w)
		return err
	}
	fmt.Fprintf(w, "feed %q refreshing\n", feeder.Feed.URL.String())
	if request, err = feeder.Request(ctx); err != nil {
		feeder.Log(nil, errors.Join(ErrRequest, err), w)
		return err
	}
	if response, err = client.Do(request); err != nil {
		feeder.Log(response, errors.Join(ErrFetch, err), w)
		return err
	} else if response.StatusCode >= 400 {
		err = fmt.Errorf("HTTP %d %s", response.StatusCode, http.StatusText(response.StatusCode))
		feeder.Log(response, errors.Join(ErrResponse, err), w)
		return err
	}
	feeder.Apply(response, w) // apply response to feed status
	if body, err = io.ReadAll(response.Body); err != nil {
		feeder.Log(response, errors.Join(ErrResponse, err), w)
		return err
	}
	return feeder.Parse(ctx, request.URL, body, w)
}

// Ready determines if the Feeder is ready to be fetched again.
func (feeder *Feeder) Ready() (ready bool, err error) {
	var tick time.Time = time.Now().UTC()

	// check feed status
	if feeder.Feed == nil {
		feeder.Feed = &hyperfeed.Feed{URL: &href.Href{}}
		return false, ErrFeed // feeder must have a feed
	} else if !feeder.Feed.Status.Healthy() {
		return false, ErrUnhealthy // feed is unhealthy; checkup required
	} else if feeder.Feed.Status.UpdatedAt.IsZero() || feeder.Feed.Status.LastSuccessAt.IsZero() {
		return true, nil // new feed
	} else if time.Time.Equal(feeder.Feed.Status.UpdatedAt, feeder.Feed.Status.LastCanceledAt) {
		return true, nil // last attempt was canceled; allow immediate retry
	}

	// apply cooldown
	cooldown := feeder.Feed.Status.Cooldown(feeder.Feed.Interval)
	retryAfter := feeder.Feed.Status.LastAttemptAt.Add(feeder.Feed.Status.RateLimit.RetryAfter)

	// check readiness (paused, rate limited, error cooldown, interval)
	// TODO: add support for checking if feeder.Feed.Interval < feeder.Feed.Status.RateLimit.PollInterval
	if tick.Before(feeder.Feed.Status.LastSuccessAt.Add(feeder.Feed.Interval)) {
		return false, ErrTooSoonJunior // not enough time has passed since last successful fetch (interval)
	}
	if tick.Before(feeder.Feed.Status.PausedUntil) {
		return false, ErrPaused // feed is paused
	}
	if tick.Before(feeder.Feed.Status.RateLimit.WaitUntil) {
		return false, ErrThrottled // throttled to respect server-side rate limits
	}
	if tick.Before(retryAfter) {
		return false, ErrThrottled // throttled to respect server-side rate limits
	}
	if tick.Before(feeder.Feed.Status.LastErrorAt.Add(cooldown)) {
		return false, ErrTooSoonJunior // let's wait a beat before trying again
	}
	return true, nil
}

// Request builds a transporter.Request for the current feed and status.
func (feeder *Feeder) Request(ctx context.Context) (request *http.Request, err error) {
	var headers http.Header = http.Header{}
	// Always set If-Modified-Since based on last successful fetch
	if !feeder.Status.LastSuccessAt.IsZero() {
		headers.Add("If-Modified-Since", feeder.Status.LastSuccessAt.UTC().Format(http.TimeFormat))
	}

	// If a previous request returned a Last-Modified header, use it for If-Modified-Since
	if !feeder.Status.LastModified.IsZero() {
		headers.Set("If-Modified-Since", feeder.Status.LastModified.UTC().Format(http.TimeFormat))
	}

	// ETag validator
	if feeder.Status.ETag != "" {
		headers.Add("If-None-Match", feeder.Status.ETag)
	}
	return Request(ctx, http.MethodGet, feeder.Feed.URL.String(), headers, nil)
}

// Apply updates the feeder state based on the transporter response.
func (feeder *Feeder) Apply(response *http.Response, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	feeder.ApplyReset(response, w)
	feeder.ApplyTimestamps(response, nil, w)
	feeder.ApplyStatusCode(response, nil, w)
	feeder.ApplyHeaders(response, w)
	feeder.ApplyRateLimits(response, w)
}
func (feeder *Feeder) ApplyReset(response *http.Response, w io.Writer) {
	feeder.Feed.Status.LastCanceledAt = time.Time{}
	feeder.Feed.Status.LastError = ""
	if response != nil && response.StatusCode >= 100 {
		feeder.Status.FetchErrors = 0 // if we received an HTTP status code, fetch was successful
	}
}
func (feeder *Feeder) ApplyTimestamps(response *http.Response, err error, w io.Writer) {
	if feeder.Feed.Status.CreatedAt.IsZero() {
		feeder.Feed.Status.CreatedAt = feeder.Feed.Status.UpdatedAt
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		feeder.Feed.Status.LastCanceledAt = feeder.Feed.Status.UpdatedAt
	}
	if response == nil {
		feeder.Feed.Status.LastErrorAt = feeder.Feed.Status.UpdatedAt
		return
	}
	if response.StatusCode >= 100 {
		feeder.Feed.Status.LastAttemptAt = feeder.Feed.Status.UpdatedAt
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < 300 {
		feeder.Feed.Status.LastSuccessAt = feeder.Feed.Status.UpdatedAt
	} else if response.StatusCode == http.StatusNotModified {
		feeder.Feed.Status.LastSuccessAt = feeder.Feed.Status.UpdatedAt
	} else if response.StatusCode >= 100 {
		feeder.Feed.Status.LastErrorAt = feeder.Feed.Status.UpdatedAt
	}
}
func (feeder *Feeder) ApplyStatusCode(response *http.Response, err error, w io.Writer) {
	if feeder.Status.LastStatusCodes == nil {
		feeder.Status.LastStatusCodes = []int{}
	}
	var limit int = min(29, len(feeder.Status.LastStatusCodes))
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		feeder.Status.LastStatusCode = 0
		feeder.Status.LastStatusCodes = append([]int{0}, feeder.Status.LastStatusCodes[:limit]...) // use 0 to indicate no status code was received
		return
	}
	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrThrottled) {
		feeder.Status.LastStatusCode = 1
		feeder.Status.LastStatusCodes = append([]int{1}, feeder.Status.LastStatusCodes[:limit]...)
		return
	}
	if response == nil {
		return
	}
	feeder.Status.LastStatusCode = response.StatusCode
	feeder.Status.LastStatusCodes = append([]int{response.StatusCode}, feeder.Status.LastStatusCodes[:limit]...)
}
func (feeder *Feeder) ApplyHeaders(response *http.Response, w io.Writer) {
	feeder.Status.Headers = response.Header.Clone()
	if etag := response.Header.Get("ETag"); etag != "" {
		feeder.Status.ETag = etag
	}
	if response.ContentLength > 0 {
		feeder.Status.ContentLength = response.ContentLength
	}
	feeder.Status.LastModified = HeaderTimeValue(response.Header, "Last-Modified")
}
func (feeder *Feeder) ApplyRateLimits(response *http.Response, w io.Writer) {
	if response == nil {
		return
	}
	feeder.Status.RateLimit.WaitUntil = HeaderTimeValue(response.Header, "Retry-After")
	feeder.Status.RateLimit.RetryAfter = HeaderDurationValue(response.Header, "Retry-After")
	feeder.Status.RateLimit.PollInterval = HeaderDurationValue(response.Header, "X-Poll-Interval")
	feeder.Status.RateLimit.Recent429s = count(feeder.Status.LastStatusCodes, func(code int) bool { return code == 429 })
	feeder.Status.RateLimit.Recent5xxs = count(feeder.Status.LastStatusCodes, func(code int) bool { return code >= 500 })
	if feeder.Status.RateLimit.Recent429s > 0 && feeder.Status.RateLimit.RetryAfter == 0 && feeder.Status.RateLimit.WaitUntil.IsZero() {
		// we received 429s but no Retry-After header
		backoff := feeder.Status.RateLimit.Recent429s * 2
		feeder.Status.RateLimit.RetryAfter = time.Duration(backoff) * time.Minute
	}
}

// Log persists failure information for the current tick.
func (feeder *Feeder) Log(response *http.Response, err error, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	feeder.ApplyReset(response, w)
	feeder.ApplyTimestamps(response, err, w)
	feeder.ApplyStatusCode(response, err, w)
	feeder.ApplyRateLimits(response, w)
	feeder.LogErrors(response, err, w)
}
func (feeder *Feeder) LogErrors(response *http.Response, err error, w io.Writer) {
	var message string = strings.Replace(fmt.Sprintf("%v", err), "\n", ": ", -1)
	switch {
	case errors.Is(err, ErrPaused):
		fmt.Fprintf(w, "feed %q paused %s\n", feeder.Feed.URL.String(), feeder.Status.PausedUntil.Format(time.RFC3339))
		return
	case errors.Is(err, ErrTooSoonJunior):
		fmt.Fprintf(w, "feed %q skipped\n", feeder.Feed.URL.String())
		return
	case errors.Is(err, ErrReadiness), errors.Is(err, ErrRequest):
		fmt.Fprintf(w, "feed %q skipped: %v\n", feeder.Feed.URL.String(), message)
		return
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		fmt.Fprintf(w, "feed %q canceled\n", feeder.Feed.URL.String())
		return
	case errors.Is(err, ErrRateLimited) || errors.Is(err, ErrThrottled):
		fmt.Fprintf(w, "feed %q rate limited: %v\n", feeder.Feed.URL.String(), message)
		return
	case errors.As(err, &ErrDNS):
		fmt.Fprintf(w, "feed %q failed\n", feeder.Feed.URL.String())
	case errors.As(err, &ErrTLSHostName), errors.As(err, &ErrTLSInvalid), errors.As(err, &ErrTLSUnknownCA):
		fmt.Fprintf(w, "feed %q failed\n", feeder.Feed.URL.String())
	case errors.As(err, &http2.StreamError{}):
		fmt.Fprintf(w, "feed %q failed with HTTP2 error: %v\n", feeder.Feed.URL.String(), message)
	case errors.Is(err, ErrFetch):
		fmt.Fprintf(w, "feed %q fetch error: %v\n", feeder.Feed.URL.String(), message)
	case errors.Is(err, ErrResponse):
		fmt.Fprintf(w, "feed %q failed\n", feeder.Feed.URL.String())
	default:
		fmt.Fprintf(w, "feed %q unknown error: %v\n", feeder.Feed.URL.String(), message)
	}
	feeder.Feed.Status.LastAttemptAt = feeder.Feed.Status.UpdatedAt
	feeder.Feed.Status.LastErrorAt = feeder.Feed.Status.UpdatedAt
	feeder.Feed.Status.FetchErrors++
	feeder.Feed.Status.LastError = message
}

func (feeder *Feeder) Parse(ctx context.Context, u *url.URL, bytes []byte, w io.Writer) (err error) {
	var parser FeedParser
	if w == nil {
		w = os.Stdout
	}
	if parser, err = NewFeedParser(feeder.Feed, bytes); err != nil || parser == nil {
		return err
	}
	if _, err = parser.Parse(w); err != nil {
		return err
	}
	// feeder.Feed = parser.Feed
	return nil
}

// FUNCTIONS
// ============================================================================

func Request(ctx context.Context, method string, href string, headers http.Header, body io.Reader) (request *http.Request, err error) {
	if ctx == nil {
		return nil, errors.Join(ErrRequest, fmt.Errorf("nil request context"))
	}
	if request, err = http.NewRequestWithContext(ctx, method, href, body); err != nil {
		return nil, err
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", version.LibUserAgent)
	}
	request.Header = headers
	return request, nil
}

func HeaderDurationValue(headers http.Header, name string) (duration time.Duration) {
	var err error
	if value := headers.Get(name); value == "" {
		return 0
	} else if duration, err = time.ParseDuration(value + "s"); err != nil {
		return 0
	}
	return duration
}

func HeaderTimeValue(headers http.Header, name string) (date time.Time) {
	var err error
	if value := headers.Get(name); value == "" {
		return time.Time{}
	} else if date, err = time.Parse(http.TimeFormat, value); err != nil {
		return time.Time{}
	}
	return date
}

func count[T any](slice []T, f func(T) bool) (count int) {
	for _, s := range slice {
		if f(s) {
			count++
		}
	}
	return count
}
