package feeder

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/database"
	"github.com/herdworks/hyperfeed-go/internal/transporter"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go"
)

// ============================ Package Types =================================

// Feeder manages a Feed.
type Feeder struct {
	*hyperfeed.Feed

	Interval time.Duration

	Clock       Clock // TODO
	Transporter *transporter.Transporter
	Database    *database.Database
}

// Clock abstracts time so tests can inject deterministic clocks.
type Clock interface {
	Now() time.Time
}

// ============================ Package Variables =============================

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
	ErrResponse       = errors.New("response error")
	ErrDatabase       = errors.New("database error")
	ErrScraper        = errors.New("scraper error")
)

// ========================= Feeder Methods =============================

// Tick computes the feeder's next event. Implementation must load
// state, honor rate-limit hints, and update persistence optimistically to avoid
// duplicate work after crashes.
func (feeder *Feeder) Tick(ctx context.Context) error {
	var ready bool
	var request *transporter.Request
	var response *transporter.Response
	var err error
	if ready, err = feeder.Ready(); !ready {
		feeder.Log(nil, errors.Join(ErrReadiness, err))
		return err
	}
	if request, err = feeder.Request(); err != nil {
		feeder.Log(nil, errors.Join(ErrRequest, err))
		return err
	}
	if feeder.Transporter == nil {
		feeder.Transporter = &transporter.Transporter{} // init a transporter
	}
	if response, err = feeder.Transporter.Fetch(ctx, request); err != nil {
		feeder.Log(response, errors.Join(ErrResponse, err))
		return err
	}
	if err = feeder.Apply(ctx, response); err != nil {
		return err
	}
	return feeder.Parse(ctx, request.URL, response.Bytes)
}

// Ready determines if the Feeder is ready to be fetched again.
func (feeder *Feeder) Ready() (ready bool, err error) {
	var tick time.Time = time.Now().UTC()
	var cooldown time.Duration
	if feeder.Feed == nil {
		feeder.Feed = &hyperfeed.Feed{URL: &href.Href{}, Status: &hyperfeed.FeedStatus{}}
		return false, ErrFeed // feeder must have a feed
	}
	if feeder.Feed.Status == nil {
		feeder.Feed.Status = &hyperfeed.FeedStatus{}
		return true, nil // new feed
	} else {
		cooldown = feeder.Feed.Status.Cooldown(feeder.Interval)
	}
	if tick.Before(feeder.Feed.Status.PausedUntil) {
		return false, ErrPaused // feed is paused
	}
	if tick.Before(feeder.Feed.Status.LastErrorAt.Add(cooldown)) {
		return false, ErrTooSoonJunior // feed is in cooldown-only period after last error
	}
	if tick.Before(feeder.Feed.Status.LastSuccessAt.Add(feeder.Interval + cooldown)) {
		return false, ErrTooSoonJunior // not enough time has passed since last successful fetch (interval+cooldown)
	}
	if tick.Before(feeder.Feed.Status.RateLimit.WaitUntil) {
		return false, ErrRateLimited // rate limited
	}
	if retryAfter := feeder.Feed.Status.LastAttemptAt.Add(feeder.Feed.Status.RateLimit.RetryAfter + cooldown); tick.Before(retryAfter) {
		return false, ErrRateLimited // rate limited
	}
	return true, nil
}

// Request builds a transporter.Request for the current feed and status.
func (feeder *Feeder) Request() (request *transporter.Request, err error) {
	if feeder.Feed == nil || feeder.Feed.URL == nil {
		return nil, errors.Join(ErrRequest, fmt.Errorf("invalid feed"))
	}
	request = &transporter.Request{
		URL:        feeder.Feed.URL.URL(),
		Conditions: feeder.Conditions(),
		Request: &http.Request{
			Header: http.Header{
				"User-Agent": []string{hyperfeed.LibUserAgent}, // set the versioned user-agent header
			},
		},
		Options: transporter.RequestOptions{}, // placeholder for future configuration
	}
	return request, nil
}

// Conditions generates a list of request conditions based on the current event state.
func (feeder *Feeder) Conditions() (conditions []transporter.Condition) {
	if feeder.Status == nil {
		return nil
	}
	// Default conditions
	conditions = append(
		conditions,
		transporter.Condition{
			Kind:      transporter.ConditionLastOK,
			TimeValue: feeder.Status.LastSuccessAt,
		},
		transporter.Condition{
			Kind:          transporter.ConditionRateLimit,
			TimeValue:     feeder.Status.RateLimit.WaitUntil,
			DurationValue: feeder.Status.RateLimit.RetryAfter,
		},
	)

	// ETag validator
	if feeder.Status.ETag != "" {
		conditions = append(conditions, transporter.Condition{
			Kind:  transporter.ConditionIfNoneMatch,
			Value: feeder.Status.ETag,
		})
	}

	// Last-Modified validator
	if !feeder.Status.LastModified.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfModifiedSince,
			TimeValue: feeder.Status.LastModified,
		})
	}

	// Rate limit guardrails
	if !feeder.Status.RateLimit.WaitUntil.IsZero() {
		conditions = append(conditions, transporter.Condition{
			Kind:      transporter.ConditionIfUnmodifiedSince,
			Name:      "X-RateLimit-Wait-Until",
			TimeValue: feeder.Status.RateLimit.WaitUntil,
		})
	}
	return conditions
}

// Apply updates the feeder state based on the transporter response.
// Implementation must update error counters, compute backoff, and persist state.
func (feeder *Feeder) Apply(ctx context.Context, response *transporter.Response) error {
	if feeder.Status == nil {
		feeder.Status = &hyperfeed.FeedStatus{LastStatusCodes: []int{}}
	}
	feeder.ApplyReset(response)
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.ApplyHeaders(response)
	feeder.ApplyRateLimits(response)
	feeder.ApplyBody(response)
	return nil
}
func (feeder *Feeder) ApplyTimestamps(response *transporter.Response) {
	// always updated UpdatedAt
	feeder.Status.UpdatedAt = response.ReceivedAt.UTC()
	if feeder.Status.CreatedAt.IsZero() {
		// create timestamps
		feeder.Status.CreatedAt = response.ReceivedAt.UTC()
	}
	if response.Attempted() {
		// attempt timestamps
		feeder.Status.LastAttemptAt = response.ReceivedAt.UTC()
	}
	if response.OK() {
		// success timestamps
		feeder.Feed.UpdatedAt = response.ReceivedAt.UTC()
		feeder.Status.LastSuccessAt = response.ReceivedAt.UTC()
	} else if response.NotModified() {
		// not modified is success, but the feed will not be marked as updated
		feeder.Status.LastSuccessAt = response.ReceivedAt.UTC()
	} else if response.Attempted() {
		// failure timestamps
		feeder.Status.LastErrorAt = response.ReceivedAt.UTC()
	}
}
func (feeder *Feeder) ApplyStatusCode(response *transporter.Response) {
	if feeder.Status.LastStatusCodes == nil {
		feeder.Status.LastStatusCodes = []int{}
	}
	var limit int = min(29, len(feeder.Status.LastStatusCodes))
	feeder.Status.LastStatusCode = response.StatusCode
	if response.StatusCode < 100 {
		return // only log valid HTTP status codes
	}
	feeder.Status.LastStatusCodes = append([]int{response.StatusCode}, feeder.Status.LastStatusCodes[:limit]...)
}
func (feeder *Feeder) ApplyHeaders(response *transporter.Response) {
	feeder.Status.Headers = response.Response.Header.Clone()
	if etag := response.Response.Header.Get("ETag"); etag != "" {
		feeder.Status.ETag = etag
	}
	if response.Response.ContentLength > 0 {
		feeder.Status.ContentLength = response.Response.ContentLength
	}
	feeder.Status.LastModified = response.LastModified()
}
func (feeder *Feeder) ApplyRateLimits(response *transporter.Response) {
	feeder.Status.RateLimit.WaitUntil = response.WaitUntil()
	feeder.Status.RateLimit.RetryAfter = response.RetryAfter()
	feeder.Status.RateLimit.PollInterval = response.PollInterval()
	feeder.Status.RateLimit.Recent429s = count(feeder.Status.LastStatusCodes, func(code int) bool { return code == 429 })
	feeder.Status.RateLimit.Recent5xxs = count(feeder.Status.LastStatusCodes, func(code int) bool { return code >= 500 })
}
func (feeder *Feeder) ApplyBody(response *transporter.Response) {
	feeder.Feed.Bytes = response.Bytes
	// TODO: parse feed body and update feeder.Status.FeedTTL or feeder.Status.RateLimit.WaitUntil?
	// TODO: parse feed body and update FeedKind?
}

func (feeder *Feeder) ApplyReset(response *transporter.Response) {
	feeder.Status.LastError = ""
	// reset fetch error counter on successful fetch
	if response.Response.StatusCode >= 100 {
		feeder.Status.FetchErrors = 0 // if we received an HTTP status code, fetch was successful
	}
}

// Log persists failure information for the current tick.
func (feeder *Feeder) Log(response *transporter.Response, err error) {
	if response == nil {
		response = &transporter.Response{
			Response:   &http.Response{},
			ReceivedAt: time.Now().UTC(),
		}
	}
	feeder.ApplyReset(response)
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.LogErrors(response, err)
}
func (feeder *Feeder) LogErrors(response *transporter.Response, err error) {
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
	case errors.Is(err, ErrRateLimited):
		fmt.Printf("feed %q rate limited: %v\n", feeder.Feed.URL.String(), message)
		return
	case errors.As(err, &transporter.ErrDNS):
		fmt.Printf("feed %q failed\n", feeder.Feed.URL.String())
	case errors.As(err, &transporter.ErrTLSHostName), errors.As(err, &transporter.ErrTLSInvalid), errors.As(err, &transporter.ErrTLSUnknownCA):
		fmt.Printf("feed %q failed\n", feeder.Feed.URL.String())
	case errors.Is(err, ErrResponse):
		fmt.Printf("feed %q failed\n", feeder.Feed.URL.String())
	default:
		fmt.Printf("feed %q unknown error: %v\n", feeder.Feed.URL.String(), message)
	}
	feeder.Feed.Status.LastAttemptAt = response.ReceivedAt.UTC()
	feeder.Feed.Status.LastErrorAt = response.ReceivedAt.UTC()
	feeder.Feed.Status.FetchErrors++
	feeder.Feed.Status.LastError = message
}

func (feeder *Feeder) Parse(ctx context.Context, u *url.URL, bytes []byte) (err error) {
	var parser FeedParser
	if bytes == nil {
		return nil // nothing to parse
	}
	if parser = NewFeedParser(u, bytes, feeder.Feed); parser == nil {
		return err
	}
	if feeder.Feed, err = parser.Parse(); err != nil {
		return err
	}
	return nil
}

// Save persists the current feed and status to the database
func (feeder *Feeder) SaveFeed(ctx context.Context) (err error) {
	if err = feeder.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = feeder.Database.UpsertRow(ctx, feeder.Feed); err != nil {
		return errors.Join(ErrDatabase, errors.New("upserting feed"), err)
	}
	for _, author := range feeder.Feed.Authors {
		if err = feeder.Database.UpsertRow(ctx, &author); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting author"), err)
		}
		columns := []string{"feed_id", "author_id"}
		values := []any{feeder.Feed.Identifier(), author.Identifier()}
		if err = feeder.Database.RelateRows(ctx, "author_relation", columns, values); err != nil {
			return errors.Join(ErrDatabase, errors.New("inserting feed-author relation"), err)
		}
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
			columns := []string{"item_id", "author_id"}
			values := []any{item.Identifier(), byline.Identifier()}
			if err = feeder.Database.RelateRows(ctx, "author_relation", columns, values); err != nil {
				return errors.Join(ErrDatabase, errors.New("inserting item-author relation"), err)
			}
		}
		for _, attachment := range item.Attachments {
			if err = feeder.Database.UpsertRow(ctx, &attachment); err != nil {
				fmt.Printf("attachment.item_id: %q\n", attachment.ItemID)
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

func (feeder *Feeder) HyperGraphEntity(ctx context.Context) (err error) {
	var document *hypergraph.Document = &hypergraph.Document{Href: feeder.Feed.HTMLURL.String()}
	for _, author := range feeder.Feed.Authors {
		if author.Source == "hypergraph" {
			return nil // already have hypergraph entity
		}
	}
	if err = document.Download(ctx, true, nil); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("downloading document"), err)
	}
	if err = document.Scrape(ctx, true, nil); err != nil {
		return errors.Join(ErrScraper, fmt.Errorf("scraping document"), err)
	}
	documentHref, _ := href.Parse(document.Metadata.Entity.Href)
	faviconHref, _ := href.Parse(document.Metadata.Entity.Favicon)
	coverHref, _ := href.Parse(document.Metadata.Entity.Cover)
	feeder.Feed.Authors = append(feeder.Feed.Authors, hyperfeed.Author{
		Href:        documentHref,
		Username:    document.Metadata.Entity.Username,
		Name:        document.Metadata.Entity.Name,
		Description: document.Metadata.Entity.Description,
		Email:       document.Metadata.Entity.Email,
		Favicon:     faviconHref,
		Cover:       coverHref,
		Verified:    document.Metadata.Entity.Verified,
		Source:      document.Metadata.Entity.Source,
	})
	return nil
}

// ========================= Helper Functions ===========================
func count[T any](slice []T, f func(T) bool) (count int) {
	for _, s := range slice {
		if f(s) {
			count++
		}
	}
	return count
}
