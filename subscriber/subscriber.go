package subscriber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/herdworks/hyperfeed-go/feeder"
	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ============================ Package Types =================================

// Subscriber manages a SubscriptionList
type Subscriber struct {
	hyperfeed.SubscriptionList

	Config   *Config
	Feeds    []*feeder.Feeder
	Database *database.Database
}

// Config holds default Feeder configuration parameters
type Config struct {
	// DefaultInterval is the baseline polling interval when no hints are available.
	// Implementations must never schedule requests more frequently than this value.
	DefaultInterval time.Duration `json:"default_interval"`

	// MinInterval constrains how aggressively we can poll, even when feeds publish explicit TTL hints.
	// Implementations must clamp decisions to this floor to avoid overwhelming origins.
	MinInterval time.Duration `json:"min_interval"`

	// MaxInterval caps how stale a feed is allowed to become.
	// Implementations must clamp decisions to this ceiling even when the origin suggests very long TTLs, unless the operator explicitly overrides the value.
	MaxInterval time.Duration `json:"max_interval"`

	// StartupGracePeriod allows newly subscribed feeds to be fetched more aggressively during onboarding without violating server hints.
	StartupGracePeriod time.Duration `json:"startup_grace_period"`
}

// ============================ Package Variables =============================

// errors
var (
	ErrNotImplemented   = errors.New("not implemented")
	ErrReadiness        = errors.New("subscriber not ready")
	ErrSubscriber       = errors.New("subscriber error")
	ErrSubscriptionList = errors.New("subscription list error")
)

// defaults
var (
	DefaultFeederConfig = &Config{
		DefaultInterval:    IntervalActivityHigh,
		MinInterval:        IntervalActivityLive,
		MaxInterval:        IntervalActivityDaily,
		StartupGracePeriod: 5 * time.Minute,
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
	if subscriber.SubscriptionList.Path == "" {
		return false, errors.Join(ErrReadiness, errors.New("subscription list URI is nil"))
	}
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

// Init initializes the Subscriber database, etc
//
// NOTE: Init uses a URI instead of a path string to allow for future flexibility (e.g. reading subscription lists from remote locations).
func (subscriber *Subscriber) Init(ctx context.Context, uri *url.URL, config *Config) (err error) {
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
	if uri != nil {
		subscriber.SubscriptionList.Path = uri.String()
		subscriber.Database.Path = filepath.Join(filepath.Dir(uri.Path), "hyperfeed.db")
	} else {
		return errors.Join(ErrSubscriptionList, errors.New("subscription list URI is nil"))
	}

	// Open the database, creating it if necessary
	if err = subscriber.Database.Open(ctx); err != nil {
		return errors.Join(ErrSubscriber, fmt.Errorf("subscriber database %q open failed", subscriber.Database.Path), err)
	}
	return subscriber.Database.UpsertRowValue(ctx, &subscriber.SubscriptionList)
}

// Load loads the SubscriptionList from its OPML URI and initializes the Subscriber
func (subscriber *Subscriber) Load(ctx context.Context) (err error) {
	if ready, err := subscriber.Ready(ctx); !ready || err != nil {
		return err
	}
	if err = subscriber.SubscriptionList.Load(); err != nil {
		return err
	}
	if err = subscriber.InitFeeds(); err != nil {
		return err
	}
	if err = subscriber.ResolveFeeds(context.Background()); err != nil {
		return err
	}
	return nil
}

// InitFeeds initializes the managed Feeds slice ([]*feeder.Feeder)
func (subscriber *Subscriber) InitFeeds() (err error) {
	for _, outline := range subscriber.OPML.Body.Outlines {
		var feeder *feeder.Feeder = &feeder.Feeder{
			Feed:     &hyperfeed.Feed{Title: outline.Text},
			Interval: subscriber.Config.DefaultInterval,
			Database: subscriber.Database,
		}
		if feeder.Feed.URL, err = url.Parse(outline.XMLURL); err != nil {
			return err
		}
		if feeder.Feed.HTMLURL, err = url.Parse(outline.HTMLURL); err != nil {
			return err
		}
		subscriber.Feeds = append(subscriber.Feeds, feeder)
	}
	return nil
}

// ResolveFeeds loads existing feed state from the database (if any)
func (subscriber *Subscriber) ResolveFeeds(ctx context.Context) (err error) {
	for _, feed := range subscriber.Feeds {
		if err := subscriber.Database.SyncRowValues(ctx, feed.Feed); err != nil {
			return err
		}
		if feed.Feed.StatusJSON == nil {
			feed.Feed.StatusJSON = []byte("{}")
		}
		if err = json.Unmarshal(feed.Feed.StatusJSON, &feed.Feed.Status); err != nil {
			return err
		}
	}
	return nil
}

// Refresh refreshes all subscribed feeds concurrently
func (subscriber *Subscriber) Refresh(ctx context.Context) (err error) {
	var ticker *errgroup.Group
	var cancellation context.Context
	var start time.Time = time.Now().UTC()
	subscriber.SubscriptionList.Status = &hyperfeed.SubscriptionStatus{UpdatedAt: time.Now().UTC()}
	ticker, cancellation = errgroup.WithContext(ctx)
	if err = subscriber.Database.Open(ctx); err == nil {
		defer subscriber.Database.Close()
	} else {
		return err
	}
	for _, feed := range subscriber.Feeds {
		f := feed // capture range variable
		ticker.Go(func() (e error) {
			fmt.Printf("feed %q refreshing\n", f.Feed.URL.String())
			f.Tick(cancellation)
			if e = f.Parse(); e == nil {
				subscriber.SubscriptionList.Status.ItemsFetched.Increment(len(f.Items))
			} else {
				return errors.Join(ErrSubscriber, e)
			}
			if e = f.SaveFeed(ctx); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if e = f.SaveItems(ctx); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if f.Feed.Status.Fetched() {
				subscriber.SubscriptionList.Status.FeedsFetched.Increment(1)
			}
			if f.Feed.Status.Skipped() {
				subscriber.SubscriptionList.Status.FeedsSkipped.Increment(1)
			}
			if f.Feed.Status.Failed() {
				subscriber.SubscriptionList.Status.FeedsFailed.Increment(1)
			}
			if a, u, err := f.AddedItems(ctx, f.Feed); err == nil {
				subscriber.SubscriptionList.Status.ItemsAdded.Increment(a)
				subscriber.SubscriptionList.Status.ItemsUpdated.Increment(u)
			} else {
				return errors.Join(ErrSubscriber, err)
			}
			if dups := f.Feed.DuplicateItems(); len(dups) > 0 {
				subscriber.SubscriptionList.Status.ItemsDuplicate.Increment(len(dups))
			}
			return nil
		})
	}
	if err = ticker.Wait(); err != nil {
		return err
	}
	subscriber.SubscriptionList.Status.Duration = time.Since(start)
	fmt.Println(subscriber.SubscriptionList.Status.Print())
	return subscriber.Database.UpsertRowValue(ctx, &subscriber.SubscriptionList)
}
