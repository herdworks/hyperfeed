package subscriber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	ErrNotImplemented = errors.New("not implemented")
	ErrSubscriber     = errors.New("subscriber error")
)

// defaults
var (
	DefaultFeederConfig = &Config{
		DefaultInterval:    1 * time.Minute,
		MinInterval:        time.Minute,
		MaxInterval:        12 * time.Hour,
		StartupGracePeriod: 5 * time.Minute,
	}
)

// ============================ Subscriber Methods ============================

// Load loads the SubscriptionList from its OPML URI and initializes the Subscriber
func (subscriber *Subscriber) Load() (err error) {
	if subscriber.Config == nil {
		cc := *DefaultFeederConfig // copy of default config
		subscriber.Config = &cc    // init subscriber.Config with defaults
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
		subscriber.Feeds = append(subscriber.Feeds, feeder)
	}
	return nil
}

// ResolveFeeds loads existing feed state from the database (if any)
func (subscriber *Subscriber) ResolveFeeds(ctx context.Context) (err error) {
	for _, feed := range subscriber.Feeds {
		fmt.Printf("feed %q resolving\n", feed.Feed.URL.String())
		if err := subscriber.Database.SyncRowValues(ctx, feed.Feed); err != nil {
			return err
		}
		if feed.Feed.StatusJSON == nil {
			feed.Feed.StatusJSON = []byte("{}")
		}
		if err = json.Unmarshal(feed.Feed.StatusJSON, &feed.Feed.Status); err != nil {
			return err
		}
		fmt.Printf("feed %q resolved\n", feed.Feed.URL.String())
	}
	return nil
}

// Refresh refreshes all subscribed feeds concurrently
func (subscriber *Subscriber) Refresh(ctx context.Context) (err error) {
	var ticker *errgroup.Group
	var cancellation context.Context
	ticker, cancellation = errgroup.WithContext(ctx)
	if err = subscriber.Database.Open(ctx); err != nil {
		return err
	}
	for _, feed := range subscriber.Feeds {
		f := feed // capture range variable
		ticker.Go(func() (e error) {
			fmt.Printf("feed %q syncing\n", f.Feed.URL.String())
			if e = f.Tick(cancellation); errors.Is(e, feeder.ErrReadiness) {
				// noop
			} else if e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if e = f.Parse(); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if e = f.SaveFeed(ctx); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			if e = f.SaveItems(ctx); e != nil {
				return errors.Join(ErrSubscriber, e)
			}
			fmt.Printf("feed %q synced %d items\n", f.Feed.URL.String(), len(f.Feed.Items))
			return nil
		})
	}
	return ticker.Wait()
}
