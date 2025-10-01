package subscriber

import (
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// Subscriber manages a SubscriptionList
type Subscriber struct {
	hyperfeed.SubscriptionList
}
