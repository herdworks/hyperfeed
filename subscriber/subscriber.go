package subscriber

import (
	"errors"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ============================ Package Types =================================

// Subscriber manages a SubscriptionList
type Subscriber struct {
	hyperfeed.SubscriptionList
}

// ============================ Package Variables =============================

// errors
var (
	ErrNotImplemented = errors.New("not implemented")
)

// ============================ Subscriber Methods ============================

func (s *Subscriber) Init() (err error) {
	return nil
}
