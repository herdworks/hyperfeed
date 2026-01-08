package subscriber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/herdworks/hyperfeed-go/feeder"
	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// TYPES
// =============================================================================

// Subscriber manages one or more SubscriptionLists
type Subscriber struct {
	Config   *Config
	Database *database.Database
	Status   *Status
}

// VARIABLES
// =============================================================================


// METHODS
// =============================================================================

// Config holds default Feeder configuration parameters
type Config struct {
	// DatabasePath is the filepath to the database used to persist feed and item state
	DatabasePath string `json:"db_path"`

	// DefaultInterval is the baseline polling interval when no hints are available.
	// Implementations must never schedule requests more frequently than this value.
	DefaultInterval time.Duration `json:"default_interval"`
}

// Status holds Subscriber runtime status information
type Status struct {
	Duration       time.Duration     `json:"duration,omitempty"`
	UpdatedAt      time.Time         `json:"updated_at,omitempty"`
	FeedsSkipped   hyperfeed.Counter `json:"feeds_skipped,omitempty"`
	FeedsFailed    hyperfeed.Counter `json:"feeds_failed,omitempty"`
	FeedsFetched   hyperfeed.Counter `json:"feeds_fetched,omitempty"`
	ItemsFetched   hyperfeed.Counter `json:"items_fetched,omitempty"`
	ItemsDuplicate hyperfeed.Counter `json:"items_duplicate,omitempty"`
	ItemsAdded     hyperfeed.Counter `json:"items_added,omitempty"`
	ItemsUpdated   hyperfeed.Counter `json:"items_updated,omitempty"`

	// TODO: add bytes downloaded
	// TODO: add image counts
}

// ============================ Package Variables =============================

// errors
var (
	ErrNotImplemented   = errors.New("not implemented")
	ErrReadiness        = errors.New("subscriber not ready")
	ErrSubscriber       = errors.New("subscriber error")
	ErrSubscriptionList = errors.New("subscription list error")
	ErrDatabase         = errors.New("database error")
	ErrScraper          = errors.New("scraper error")
)

// defaults
var (
	DefaultFeederConfig = &Config{
		DefaultInterval: IntervalActivityHigh,
	}
	IntervalActivityLive   = 1 * time.Minute
	IntervalActivityHigh   = 10 * time.Minute
	IntervalActivityMedium = 60 * time.Minute
	IntervalActivityLow    = 240 * time.Minute  // 4 hours
	IntervalActivityDaily  = 1440 * time.Minute // 24 hours
)

// ============================ Subscriber Methods ============================

// Ready checks if the Subscriber is properly initialized
func (subscriber *Subscriber) Ready(ctx context.Context) (ready bool, err error) {
	// if subscriber.SubscriptionList.Path == "" {
	// 	return false, errors.Join(ErrReadiness, errors.New("subscription list URI is nil"))
	// }
	if subscriber.Config == nil {
		return false, errors.Join(ErrReadiness, errors.New("subscriber configuration is nil"))
	}
	if subscriber.Database == nil {
		return false, errors.Join(ErrReadiness, errors.New("subscriber database is nil"))
	}
	if err = subscriber.Database.PingContext(ctx); err != nil {
		return false, errors.Join(ErrReadiness, fmt.Errorf("subscriber database %q is unreachable", subscriber.Database.Path), err)
	}
	return true, nil
}

// Init initializes the Subscriber database
func (subscriber *Subscriber) Init(ctx context.Context, config *Config) (err error) {
	if subscriber.Database == nil {
		subscriber.Database = &database.Database{} // init empty database
	}
	if subscriber.Config == nil {
		subscriber.Config = config // use provided config
	}
	if subscriber.Config == nil {
		cc := *DefaultFeederConfig // copy of default config
		subscriber.Config = &cc    // init subscriber.Config with defaults
	}
	if subscriber.Config.DatabasePath == "" {
		if pwd, err := os.Getwd(); err == nil {
			subscriber.Config.DatabasePath = filepath.Join(pwd, "hyperfeed.db") // default to current working directory
		}
	}
	subscriber.Database.Path = filepath.Clean(subscriber.Config.DatabasePath)

	// Open the database, creating it if necessary
	if err = subscriber.Database.Open(ctx); err != nil {
		return errors.Join(ErrSubscriber, fmt.Errorf("subscriber database %q open failed", subscriber.Database.Path), err)
	}
	if err = subscriber.Database.Initialize(ctx); err != nil {
		return errors.Join(ErrSubscriber, fmt.Errorf("subscriber database %q initialize failed", subscriber.Database.Path), err)
	}
	return nil
}

// Load loads the SubscriptionList from its OPML URI and updates the feeds table
func (subscriber *Subscriber) LoadSubscriptions(ctx context.Context, subscriptionsPath string, w io.Writer) (feeds []*hyperfeed.Feed, err error) {
	var subscriptions *hyperfeed.SubscriptionList = &hyperfeed.SubscriptionList{Path: subscriptionsPath}
	if w == nil {
		w = os.Stdout
	}
	if ready, err := subscriber.Ready(ctx); !ready || err != nil {
		return nil, err
	}
	if err = subscriptions.Load(); err != nil {
		return nil, err
	}
	if err = subscriber.SaveSubscriptionList(ctx, subscriptions); err != nil {
		return nil, err
	}
	if feeds, err = subscriber.InitFeeds(subscriptions, w); err != nil {
		return nil, err
	}
	if feeds, err = subscriber.ResolveFeeds(context.Background(), feeds); err != nil {
		return nil, err
	}
	if err = subscriber.SaveFeeds(ctx, feeds, w); err != nil {
		return nil, err
	}
	return feeds, nil
}

func (subscriber *Subscriber) LoadFeeds(ctx context.Context, subscriptionsPath string, w io.Writer) (feeds []*hyperfeed.Feed, err error) {
	var subscriptions *hyperfeed.SubscriptionList = &hyperfeed.SubscriptionList{Path: subscriptionsPath}
	if w == nil {
		w = os.Stdout
	}
	if ready, err := subscriber.Ready(ctx); !ready || err != nil {
		return nil, err
	}
	if err = subscriptions.Load(); err != nil {
		return nil, err
	}
	if feeds, err = subscriber.InitFeeds(subscriptions, w); err != nil {
		return nil, err
	}
	if feeds, err = subscriber.ResolveFeeds(context.Background(), feeds); err != nil {
		return nil, err
	}
	if err = subscriber.SaveSubscriptionList(ctx, subscriptions); err != nil {
		return nil, err
	}
	return feeds, nil
}

// InitFeeders generates a Feeder per SubscriptionList.OPML.Body.Outline in the managed Feeds slice ([]*feeder.Feeder)
func (subscriber *Subscriber) InitFeeds(subscriptions *hyperfeed.SubscriptionList, w io.Writer) (feeds []*hyperfeed.Feed, err error) {
	if w == nil {
		w = os.Stdout
	}
	for _, outline := range subscriptions.OPML.Body.Outlines {
		var feed *hyperfeed.Feed = &hyperfeed.Feed{
			Title:              outline.Text,
			SubscriptionListID: subscriptions.Identifier(),
		}
		if feed.URL, err = href.Parse(outline.XMLURL); err != nil {
			return feeds, err
		}
		if outline.HTMLURL != "" {
			feed.HTMLURL, _ = href.Parse(outline.HTMLURL)
			feed.HTMLURL = href.ResolveHref(feed.URL, feed.HTMLURL, w)
		}
		feed.Status.LastStatusCodes = []int{}
		feeds = append(feeds, feed)
	}
	return feeds, nil
}

// ResolveFeeders loads existing feed state from the database (if any)
//
// NOTE: The only required field to resolve a given feed is the feed.URL, which is used by feed.Identifier() to look up existing records.
func (subscriber *Subscriber) ResolveFeeds(ctx context.Context, feeds []*hyperfeed.Feed) ([]*hyperfeed.Feed, error) {
	for _, feed := range feeds {
		if err := subscriber.Database.ReadRow(ctx, feed); err != nil {
			return nil, err
		}
	}
	return feeds, nil
}

// func (subscriber *Subscriber) GraphFeeds(ctx context.Context, feeds []*hyperfeed.Feed) ([]*hyperfeed.Feed, error) {
// 	var jobs sync.WaitGroup = sync.WaitGroup{}
// 	jobs.Add(len(feeds))
// 	for _, feed := range feeds {
// 		go func(f *hyperfeed.Feed) {
// 			defer jobs.Done()
// 			f.HyperGraphEntity(ctx)
// 		}(feed)
// 	}
// 	jobs.Wait()
// 	return feeds, nil
// }

// SaveSubscriptionList persists the SubscriptionList to the database
func (subscriber *Subscriber) SaveSubscriptionList(ctx context.Context, subscriptions *hyperfeed.SubscriptionList) (err error) {
	if err = subscriber.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = subscriber.Database.UpsertRow(ctx, subscriptions); err != nil {
		return errors.Join(ErrDatabase, errors.New("upserting subscription list"), err)
	}
	return nil
}

// SaveFeeds persists the provided feeds to the database
// NOTE: Subscriber.SaveFeeds saves feeds only; it does not save associated Websites
func (subscriber *Subscriber) SaveFeeds(ctx context.Context, feeds []*hyperfeed.Feed, _ io.Writer) (err error) {
	if err = subscriber.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	for _, feed := range feeds {
		if err = subscriber.Database.UpsertRow(ctx, feed); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting feed"), err)
		}
	}
	return nil
}

// Refresh refreshes all subscribed feeds concurrently
// TODO: can this be split up into smaller methods for testability?
func (subscriber *Subscriber) Refresh(ctx context.Context, timelinePosition time.Time, feeds []*hyperfeed.Feed, timeout int, w io.Writer) (err error) {
	var client *http.Client = feeder.Client(feeder.Config{Timeout: time.Duration(timeout) * time.Second})
	var ticker *errgroup.Group
	var cancellation context.Context
	var start time.Time = time.Now().UTC()
	subscriber.Status = &Status{UpdatedAt: time.Now().UTC()}
	ticker, cancellation = errgroup.WithContext(ctx)
	if w == nil {
		w = os.Stdout
	}

	// TODO: do we need to open the database here, or can we just check if it is already open?
	if err = subscriber.Database.Open(ctx); err == nil {
		defer subscriber.Database.Close()
	} else {
		return err
	}

	// Resolve provided feeds with state from database
	if feeds, err = subscriber.ResolveFeeds(ctx, feeds); err != nil {
		return err
	}

	// If no feeds provided, load all feeds in database
	if len(feeds) == 0 {
		if feeds, err = database.ListRecords[*hyperfeed.Feed](ctx, subscriber.Database); err != nil {
			return err
		}
	}

	// Refresh all feeds concurrently
	for _, feed := range feeds {
		ff := &feeder.Feeder{
			Feed:     feed,
			Database: subscriber.Database,
		}
		if ff.Feed.Interval == 0 {
			ff.Feed.Interval = 5 * time.Minute // default interval
		}
		ticker.Go(func() (e error) {
			ff.Tick(cancellation, client, timelinePosition, w) // ignore tick errors; they are recorded in f.Feed.Status
			subscriber.Status.ItemsFetched.Increment(len(ff.Feed.Items))
			if ff.Feed.Status.LastStatusCode == 200 && time.Since(ff.Feed.Status.LastEntityUpdateAt) > 24*time.Hour {
				if e = ff.Feed.HyperGraphEntity(ctx, timeout, w); e != nil {
					fmt.Fprintf(w, "graph %q website error: %v\n", ff.Feed.HTMLURL.String(), e)
					ff.Feed.Status.LastEntityError = e.Error()
				}
				ff.Feed.Status.LastEntityUpdateAt = time.Now().UTC()
			}
			if e = ff.SaveFeed(ctx, w); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if e = ff.SaveItems(ctx, w); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if ff.Feed.Status.Fetched() {
				subscriber.Status.FeedsFetched.Increment(1)
			}
			if ff.Feed.Status.Skipped() {
				subscriber.Status.FeedsSkipped.Increment(1)
			}
			if ff.Feed.Status.Failed() {
				subscriber.Status.FeedsFailed.Increment(1)
			}
			if a, u, err := ff.AddedItems(ctx, ff.Feed, w); err == nil {
				subscriber.Status.ItemsAdded.Increment(a)
				subscriber.Status.ItemsUpdated.Increment(u)
			} else {
				return errors.Join(ErrSubscriber, err)
			}
			if dups := ff.Feed.DuplicateItems(); len(dups) > 0 {
				subscriber.Status.ItemsDuplicate.Increment(len(dups))
			}
			return nil
		})
	}
	if err = ticker.Wait(); err != nil {
		return err
	}
	subscriber.Status.Duration = time.Since(start)
	fmt.Fprintln(w, subscriber.Status.Print())
	return nil
}

// ======================= SubscriptionStatus Methods =========================

func (status *Status) Print() string {
	var report string = "\n"
	report += fmt.Sprintf("Feeds Skipped:   %6d\n", status.FeedsSkipped.Count)
	report += fmt.Sprintf("Feeds Failed:    %6d\n", status.FeedsFailed.Count)
	report += fmt.Sprintf("Feeds Fetched:   %6d\n", status.FeedsFetched.Count)
	report += fmt.Sprintf("Items Fetched:   %6d\n", status.ItemsFetched.Count)
	report += fmt.Sprintf("Items Duplicate: %6d\n", status.ItemsDuplicate.Count)
	report += fmt.Sprintf("Items Updated:   %6d\n", status.ItemsUpdated.Count)
	report += fmt.Sprintf("Items Added:     %6d\n", status.ItemsAdded.Count)
	report += "-----------------------\n"
	report += fmt.Sprintf("TOTAL:           %6v\n", status.Duration.Truncate(time.Millisecond))
	return report
}

// Status JSON marshal method to convert time.duration to string
func (status *Status) MarshalJSON() (jsonBytes []byte, err error) {
	type StatusJSON Status
	s := &struct {
		Duration string `json:"duration,omitempty"`
		*StatusJSON
	}{
		Duration: status.Duration.String(),
		StatusJSON:    (*StatusJSON)(status),
	}
	return json.Marshal(s)
}