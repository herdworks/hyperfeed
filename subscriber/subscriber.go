package subscriber

import (
	"errors"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ============================ Package Types =================================

// Subscriber manages a SubscriptionList
type Subscriber struct {
	hyperfeed.SubscriptionList

	Store *database.Database
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

func (s *Subscriber) Init() (err error) {
	return nil
}
