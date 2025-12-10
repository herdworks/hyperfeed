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
	"strings"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
	"golang.org/x/net/http2"
)

// TYPES
// ============================================================================

// Feeder manages a Feed.
type Feeder struct {
	*hyperfeed.Feed

	Database *database.Database
}

// Config captures tunable knobs for feeder HTTP client behavior.
type Config struct {
	// Timeout specifies the default timeout for HTTP requests.
	// If zero, the implementation must fall back to a safe default that balances resiliency with server friendliness.
	Timeout time.Duration

	// MaxRedirects constrains the number of redirects followed for a single request.
	// This protects origin servers from loops and prevents the transporter from amplifying traffic.
	MaxRedirects int

	// MaxContentLength caps the number of bytes that will be buffered in memory for a single response.
	// Implementations must stream to disk or abort once this limit is reached.
	MaxContentLength int64
}

// VARIABLES
// ============================================================================

// errors
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrReadiness      = errors.New("feeder not ready")
	ErrPaused         = errors.New("feed paused")
	ErrTooSoonJunior  = errors.New("too soon, junior")
	ErrRateLimited    = errors.New("rate limited")
	ErrFeed           = errors.New("invalid feed")
	ErrFeedParser     = errors.New("feed parser error")
	ErrRequest        = errors.New("request error")
	ErrFetch          = errors.New("fetch error")
	ErrResponse       = errors.New("response error")
	ErrDatabase       = errors.New("database error")
	ErrDNS            *net.DNSError
	ErrTLSHostName    x509.HostnameError
	ErrTLSInvalid     x509.CertificateInvalidError
	ErrTLSUnknownCA   x509.UnknownAuthorityError

	DefaultConfig = &Config{
		Timeout:          10 * time.Second,
		MaxRedirects:     1,
		MaxContentLength: 4 << 20, // 4 MiB
	}

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
func (feeder *Feeder) Tick(ctx context.Context, client *http.Client, timelinePosition time.Time) error {
	var ready bool
	var request *http.Request
	var response *http.Response
	var body []byte
	var err error
	if ready, err = feeder.Ready(); !ready {
		feeder.Log(nil, errors.Join(ErrReadiness, err))
		return err
	}
	fmt.Printf("feed %q refreshing\n", feeder.Feed.URL.String())
	if request, err = feeder.Request(ctx); err != nil {
		feeder.Log(nil, errors.Join(ErrRequest, err))
		return err
	}
	if response, err = client.Do(request); err != nil {
		feeder.Log(response, errors.Join(ErrFetch, err))
		return err
	} else if response.StatusCode >= 400 {
		err = fmt.Errorf("HTTP %d %s", response.StatusCode, http.StatusText(response.StatusCode))
		feeder.Log(response, errors.Join(ErrResponse, err))
		return err
	}
	feeder.Apply(response) // apply response to feed status
	if body, err = io.ReadAll(response.Body); err != nil {
		feeder.Log(response, errors.Join(ErrResponse, err))
		return err
	}
	return feeder.Parse(ctx, request.URL, body, timelinePosition)
}

// Ready determines if the Feeder is ready to be fetched again.
func (feeder *Feeder) Ready() (ready bool, err error) {
	var tick time.Time = time.Now().UTC()

	// check feed status
	if feeder.Feed == nil {
		feeder.Feed = &hyperfeed.Feed{URL: &href.Href{}}
		return false, ErrFeed // feeder must have a feed
	}
	if feeder.Feed.Status.UpdatedAt.IsZero() {
		return true, nil // new feed
	}

	// apply cooldown
	cooldown := feeder.Feed.Status.Cooldown(feeder.Feed.Interval)
	retryAfter := feeder.Feed.Status.LastAttemptAt.Add(feeder.Feed.Status.RateLimit.RetryAfter)

	// check readiness (paused, rate limited, error cooldown, interval)
	// TODO: add support for checking if feeder.Feed.Interval < feeder.Feed.Status.RateLimit.PollInterval
	if tick.Before(feeder.Feed.Status.PausedUntil) {
		return false, ErrPaused // feed is paused
	}
	if tick.Before(feeder.Feed.Status.RateLimit.WaitUntil) {
		return false, ErrRateLimited // rate limited
	}
	if tick.Before(retryAfter.Add(cooldown)) {
		return false, ErrRateLimited // rate limited
	}
	if tick.Before(feeder.Feed.Status.LastErrorAt.Add(cooldown)) {
		return false, ErrTooSoonJunior // feed is in cooldown-only period after last error
	}
	if tick.Before(feeder.Feed.Status.LastSuccessAt.Add(feeder.Feed.Interval)) {
		return false, ErrTooSoonJunior // not enough time has passed since last successful fetch (interval+cooldown)
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
// Implementation must update error counters, compute backoff, and persist state.
func (feeder *Feeder) Apply(response *http.Response) {
	feeder.ApplyReset(response)
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.ApplyHeaders(response)
	feeder.ApplyRateLimits(response)
}
func (feeder *Feeder) ApplyReset(response *http.Response) {
	feeder.Status.LastError = ""
	if response != nil && response.StatusCode >= 100 {
		feeder.Status.FetchErrors = 0 // if we received an HTTP status code, fetch was successful
	}
}
func (feeder *Feeder) ApplyTimestamps(response *http.Response) {
	feeder.Feed.Status.UpdatedAt = time.Now().UTC()
	if feeder.Feed.Status.CreatedAt.IsZero() {
		feeder.Feed.Status.CreatedAt = feeder.Feed.Status.UpdatedAt
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
func (feeder *Feeder) ApplyStatusCode(response *http.Response) {
	if feeder.Status.LastStatusCodes == nil {
		feeder.Status.LastStatusCodes = []int{}
	}
	var limit int = min(29, len(feeder.Status.LastStatusCodes))
	if response == nil || response.StatusCode < 100 {
		return // only log valid HTTP status codes
	}
	feeder.Status.LastStatusCode = response.StatusCode
	feeder.Status.LastStatusCodes = append([]int{response.StatusCode}, feeder.Status.LastStatusCodes[:limit]...)
}
func (feeder *Feeder) ApplyHeaders(response *http.Response) {
	feeder.Status.Headers = response.Header.Clone()
	if etag := response.Header.Get("ETag"); etag != "" {
		feeder.Status.ETag = etag
	}
	if response.ContentLength > 0 {
		feeder.Status.ContentLength = response.ContentLength
	}
	feeder.Status.LastModified = HeaderTimeValue(response.Header, "Last-Modified")
}
func (feeder *Feeder) ApplyRateLimits(response *http.Response) {
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
func (feeder *Feeder) Log(response *http.Response, err error) {
	feeder.ApplyReset(response)
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.ApplyRateLimits(response)
	feeder.LogErrors(response, err)
}
func (feeder *Feeder) LogErrors(response *http.Response, err error) {
	var message string = strings.Replace(fmt.Sprintf("%v", err), "\n", ": ", -1)
	switch {
	case errors.Is(err, ErrPaused):
		fmt.Printf("feed %q paused %s\n", feeder.Feed.URL.String(), feeder.Status.PausedUntil.Format(time.RFC3339))
		return
	case errors.Is(err, ErrTooSoonJunior):
		fmt.Printf("feed %q skipped\n", feeder.Feed.URL.String())
		return
	case errors.Is(err, ErrReadiness), errors.Is(err, ErrRequest):
		fmt.Printf("feed %q skipped: %v\n", feeder.Feed.URL.String(), message)
		return
	case errors.As(err, &ErrDNS):
		fmt.Printf("feed %q failed\n", feeder.Feed.URL.String())
	case errors.As(err, &ErrTLSHostName), errors.As(err, &ErrTLSInvalid), errors.As(err, &ErrTLSUnknownCA):
		fmt.Printf("feed %q failed\n", feeder.Feed.URL.String())
	case errors.As(err, &http2.StreamError{}):
		fmt.Printf("feed %q failed with HTTP2 error: %v\n", feeder.Feed.URL.String(), message)
	case errors.Is(err, ErrFetch):
		fmt.Printf("feed %q fetch error: %v\n", feeder.Feed.URL.String(), message)
	case errors.Is(err, ErrRateLimited):
		fmt.Printf("feed %q rate limited: %v\n", feeder.Feed.URL.String(), message)
		return
	case errors.Is(err, ErrResponse):
		fmt.Printf("feed %q failed\n", feeder.Feed.URL.String())
	default:
		fmt.Printf("feed %q unknown error: %v\n", feeder.Feed.URL.String(), message)
	}
	feeder.Feed.Status.LastAttemptAt = feeder.Feed.Status.UpdatedAt
	feeder.Feed.Status.LastErrorAt = feeder.Feed.Status.UpdatedAt
	feeder.Feed.Status.FetchErrors++
	feeder.Feed.Status.LastError = message
}

func (feeder *Feeder) Parse(ctx context.Context, u *url.URL, bytes []byte, timelinePosition time.Time) (err error) {
	var parser FeedParser
	if parser, err = NewFeedParser(feeder.Feed, bytes); err != nil || parser == nil {
		return err
	}
	if _, err = parser.Parse(timelinePosition); err != nil {
		return err
	}
	// feeder.Feed = parser.Feed
	return nil
}

// Save persists the current feed and status to the database
func (feeder *Feeder) SaveFeed(ctx context.Context) (err error) {
	if err = feeder.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = feeder.Database.UpsertRow(ctx, &feeder.Feed.Website); err != nil {
		return errors.Join(ErrDatabase, errors.New("upserting website"), err)
	}
	if err = feeder.Database.UpsertRow(ctx, feeder.Feed); err != nil {
		return errors.Join(ErrDatabase, errors.New("upserting feed"), err)
	}
	return nil
}
func (feeder *Feeder) SaveItems(ctx context.Context) (err error) {
	if err = feeder.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	for _, item := range feeder.Items {
		if err = feeder.Database.UpsertRow(ctx, item); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting item"), err)
		}
		for _, byline := range item.Bylines {
			if err = feeder.Database.UpsertRow(ctx, &byline); err != nil {
				return errors.Join(ErrDatabase, errors.New("upserting byline"), err)
			}
			columns := []string{"feed_id", "item_id", "byline_id"}
			values := []any{item.Feed.Identifier(), item.Identifier(), byline.Identifier()}
			if err = feeder.Database.RelateRows(ctx, "item_bylines", columns, values); err != nil {
				return errors.Join(ErrDatabase, errors.New("inserting item-byline relation"), err)
			}
		}
		for _, image := range item.Images {
			image.FeedID = item.Feed.Identifier()
			image.WebsiteID = item.Feed.Website.ID
			if err = feeder.Database.InsertRow(ctx, image); err != nil {
				fmt.Printf("image.item_id: %q\n", image.ItemID)
				return errors.Join(ErrDatabase, errors.New("inserting image"), err)
			}
		}
		for _, attachment := range item.Attachments {
			attachment.FeedID = item.Feed.Identifier()
			attachment.WebsiteID = item.Feed.Website.Identifier()
			if err = feeder.Database.UpsertRow(ctx, &attachment); err != nil {
				fmt.Printf("attachment.item_id: %v\n", attachment.ItemID)
				fmt.Printf("attachment.feed_id: %v\n", attachment.FeedID)
				fmt.Printf("attachment.website_id: %v\n", attachment.WebsiteID)
				return errors.Join(ErrDatabase, errors.New("upserting attachment"), err)
			}
		}
	}
	return nil
}
func (feeder *Feeder) AddedItems(ctx context.Context, feed *hyperfeed.Feed) (int, int, error) {
	var expected int = len(feed.Items) - len(feed.DuplicateItems())
	var addedCount int = 0
	var addedQuery string = `SELECT COUNT(*) FROM items WHERE feed_id = ? AND added_at = ?;`
	var updatedCount int = 0
	var updatedQuery string = `SELECT COUNT(*) FROM items WHERE feed_id = ? AND updated_at = ? AND added_at != ?;`
	var err error
	if len(feed.Items) == 0 {
		return 0, 0, nil
	}
	if err = feeder.Database.PingContext(ctx); err != nil {
		return 0, 0, errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = feeder.Database.QueryRowContext(ctx, addedQuery, feed.ID, feed.Status.UpdatedAt.UTC().Format(time.RFC3339)).Scan(&addedCount); err != nil {
		return 0, 0, errors.Join(ErrDatabase, errors.New("counting new items"), err)
	}
	if err = feeder.Database.QueryRowContext(ctx, updatedQuery, feed.ID, feed.Status.UpdatedAt.UTC().Format(time.RFC3339), feed.Status.UpdatedAt.UTC().Format(time.RFC3339)).Scan(&updatedCount); err != nil {
		return 0, 0, errors.Join(ErrDatabase, errors.New("counting modified items"), err)
	}
	if addedCount+updatedCount != expected {
		fmt.Printf("feed %q expected %d added items, got %d\n", feed.URL.String(), expected, addedCount+updatedCount)
	}
	return addedCount, updatedCount, nil
}

// FUNCTIONS
// ============================================================================
func Client(config Config) (client *http.Client) {
	client = &http.Client{}
	if config.Timeout > 0 {
		client.Timeout = config.Timeout
	}
	return client
}

func Request(ctx context.Context, method string, href string, headers http.Header, body io.Reader) (request *http.Request, err error) {
	if ctx == nil {
		return nil, errors.Join(ErrRequest, fmt.Errorf("nil request context"))
	}
	if request, err = http.NewRequestWithContext(ctx, method, href, body); err != nil {
		return nil, err
	}
	request.Header = http.Header{
		"User-Agent": []string{hyperfeed.LibUserAgent}, // set the versioned user-agent header
		"Accept":     DefaultAcceptHeaders,
	}
	request.Header = mergeHeaders(request.Header, headers)
	return request, nil
}

// mergeHeaders combines multiple http.Header maps into one.
// Duplicate header names are case-insensitively deduplicated (last one wins).
func mergeHeaders(incoming ...http.Header) http.Header {
	var merged http.Header = http.Header{}
	for _, headers := range incoming {
		if headers == nil {
			continue
		}
		for name, values := range headers {
			var exists = merged.Get(name) != ""
			name = http.CanonicalHeaderKey(name)
			for index, value := range values {
				if exists && index == 0 {
					merged.Set(name, value) // replace existing value
				} else {
					merged.Add(name, value) // append value
				}
			}
		}
	}
	return merged
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
