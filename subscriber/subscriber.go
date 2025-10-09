package subscriber

import (
	"context"
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
		DefaultInterval:    15 * time.Minute,
		MinInterval:        time.Minute,
		MaxInterval:        12 * time.Hour,
		StartupGracePeriod: 5 * time.Minute,
	}
)

// ============================ Subscriber Methods ============================

func (subscriber *Subscriber) Init() (err error) {
	for _, outline := range subscriber.OPML.Body.Outlines {
		var feeder *feeder.Feeder = &feeder.Feeder{
			Feed:     &hyperfeed.Feed{Title: outline.Text},
			Database: subscriber.Database,
		}
		if feeder.Feed.URL, err = url.Parse(outline.XMLURL); err != nil {
			return err
		}
		subscriber.Feeds = append(subscriber.Feeds, feeder)
	}
	return nil
}

func (subscriber *Subscriber) Sync(ctx context.Context) (err error) {
	var ticker *errgroup.Group
	var cancellation context.Context
	ticker, cancellation = errgroup.WithContext(ctx)
	if err = subscriber.Database.Open(ctx); err != nil {
		return err
	}
	for _, feed := range subscriber.Feeds {
		f := feed // capture range variable
		ticker.Go(func() error {
			fmt.Printf("feed %q syncing\n", f.Feed.URL.String())
			if err = f.Tick(cancellation); err != nil {
				return err
			}
			if err = f.Parse(); err != nil {
				return err
			}
			if err = f.SaveFeed(ctx); err != nil {
				return err
			}
			if err = f.SaveItems(ctx); err != nil {
				return err
			}
			fmt.Printf("feed %q synced %d items\n", f.Feed.URL.String(), len(f.Feed.Items))
			return nil
		})
	}
	return ticker.Wait()
}
