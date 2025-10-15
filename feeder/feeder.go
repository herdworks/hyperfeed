package feeder

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"

	"github.com/herdworks/hyperfeed-go/internal/database"
	"github.com/herdworks/hyperfeed-go/internal/transporter"
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
	ErrRequest        = errors.New("request error")
	ErrResponse       = errors.New("response error")
	ErrDatabase       = errors.New("database error")
)

// ========================= Feeder Methods =============================

// Tick computes the feeder's next event. Implementation must load
// state, honor rate-limit hints, and update persistence optimistically to avoid
// duplicate work after crashes.
func (feeder *Feeder) Tick(ctx context.Context) {
	var request *transporter.Request
	var response *transporter.Response
	var err error
	if ready, err := feeder.Ready(); !ready {
		feeder.Log(nil, errors.Join(ErrReadiness, err))
		return
	}
	if request, err = feeder.Request(); err != nil {
		feeder.Log(nil, errors.Join(ErrRequest, err))
		return
	}
	if feeder.Transporter == nil {
		feeder.Transporter = &transporter.Transporter{} // init a transporter
	}
	if response, err = feeder.Transporter.Fetch(ctx, request); err != nil {
		feeder.Log(response, errors.Join(ErrResponse, err))
		return
	}
	feeder.Apply(ctx, response)
}

// Ready determines if the Feeder is ready to be fetched again.
func (feeder *Feeder) Ready() (ready bool, err error) {
	var tick time.Time = time.Now().UTC()
	var cooldown time.Duration = feeder.Status.Cooldown(feeder.Interval)
	if feeder.Feed == nil {
		return false, ErrFeed // feeder must have a feed
	}
	if feeder.Feed.Status == nil {
		return true, nil // new feed
	}
	if tick.Before(feeder.Status.PausedUntil) {
		return false, ErrPaused // feed is paused
	}
	if tick.Before(feeder.Status.UpdatedAt.Add(cooldown)) {
		return false, ErrTooSoonJunior // feed is in cooldown period after last update
	}
	if tick.Before(feeder.Status.LastSuccessAt.Add(feeder.Interval + cooldown)) {
		return false, ErrTooSoonJunior // not enough time has passed since last successful fetch
	}
	if tick.Before(feeder.Status.RateLimit.WaitUntil) {
		return false, ErrRateLimited // rate limited
	}
	if retryAfter := feeder.Status.LastAttemptAt.Add(feeder.Status.RateLimit.RetryAfter + cooldown); tick.Before(retryAfter) {
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
		URL:        feeder.Feed.URL,
		Conditions: feeder.Conditions(),
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
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.ApplyHeaders(response)
	feeder.ApplyRateLimits(response)
	feeder.ApplyBody(response)
	return nil
}
func (feeder *Feeder) ApplyTimestamps(response *transporter.Response) {
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
		feeder.Feed.Updated = response.ReceivedAt.UTC()
		feeder.Status.LastSuccessAt = response.ReceivedAt.UTC()
	} else if response.NotModified() {
		// not modified is success, but the feed will not be marked as updated
		feeder.Status.LastSuccessAt = response.ReceivedAt.UTC()
	} else {
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

// Log persists failure information for the current tick.
func (feeder *Feeder) Log(response *transporter.Response, err error) {
	if response == nil {
		response = &transporter.Response{
			Response:   &http.Response{StatusCode: 1},
			ReceivedAt: time.Now().UTC(),
		}
	}
	feeder.ApplyTimestamps(response)
	feeder.ApplyStatusCode(response)
	feeder.LogErrors(err)
}
func (feeder *Feeder) LogErrors(err error) {
	var message string = strings.Replace(fmt.Sprintf("%v", err), "\n", ": ", -1)
	feeder.Feed.Status.LastError = ""
	switch {
	case errors.Is(err, ErrPaused):
		fmt.Printf("feed %q paused %s\n", feeder.Feed.URL.String(), feeder.Status.PausedUntil.Format(time.RFC3339))
	case errors.Is(err, ErrTooSoonJunior):
		fmt.Printf("feed %q skipped\n", feeder.Feed.URL.String())
	case errors.Is(err, ErrReadiness), errors.Is(err, ErrRequest):
		fmt.Printf("feed %q skipped: %v\n", feeder.Feed.URL.String(), message)
	case errors.Is(err, ErrRateLimited):
		fmt.Printf("feed %q rate limited: %v\n", feeder.Feed.URL.String(), message)
	case errors.Is(err, ErrResponse):
		fmt.Printf("feed %q fetch error: %v\n", feeder.Feed.URL.String(), message)
		feeder.Feed.Status.LastError = message
	default:
		fmt.Printf("feed %q unknown error: %v\n", feeder.Feed.URL.String(), message)
		feeder.Feed.Status.LastError = message
	}
}

// Save persists the current feed and status to the database
func (feeder *Feeder) SaveFeed(ctx context.Context) (err error) {
	if err = feeder.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = feeder.Database.UpsertRowValue(ctx, feeder.Feed); err != nil {
		return errors.Join(ErrDatabase, errors.New("upserting feed"), err)
	}
	for _, author := range feeder.Feed.Authors {
		if err = feeder.Database.UpsertRowValue(ctx, &author); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting author"), err)
		}
		if err = feeder.Database.InsertRelationValue(ctx, feeder.Feed, &author); err != nil {
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
		if err = feeder.Database.UpsertRowValue(ctx, item); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting item"), err)
		}
		for _, author := range item.Authors {
			if err = feeder.Database.UpsertRowValue(ctx, &author); err != nil {
				return errors.Join(ErrDatabase, errors.New("upserting author"), err)
			}
			if err = feeder.Database.InsertRelationValue(ctx, item, &author); err != nil {
				return errors.Join(ErrDatabase, errors.New("inserting item-author relation"), err)
			}
		}
		for _, attachment := range item.Attachments {
			if err = feeder.Database.UpsertRowValue(ctx, &attachment); err != nil {
				fmt.Printf("attachment.item_id: %q\n", attachment.ItemID)
				return errors.Join(ErrDatabase, errors.New("upserting attachment"), err)
			}
		}
	}
	return nil
}
func (feeder *Feeder) AddedItems(ctx context.Context, feed *hyperfeed.Feed) (count int, err error) {
	var expected int = len(feed.Items) - len(feed.DuplicateItems())
	var query string = `SELECT COUNT(*) FROM items WHERE feed_id = ? AND added = ?;`
	if len(feed.Items) == 0 {
		return 0, nil
	}
	if err = feeder.Database.PingContext(ctx); err != nil {
		return 0, errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = feeder.Database.QueryRowContext(ctx, query, feed.ID, feed.Status.UpdatedAt.UTC().Format(time.RFC3339)).Scan(&count); err != nil {
		return 0, errors.Join(ErrDatabase, errors.New("counting new items"), err)
	}
	if count != expected {
		fmt.Printf("feed %q expected %d added items, got %d\n", feed.URL.String(), expected, count)
	}
	return count, nil
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
