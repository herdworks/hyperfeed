package discovery

import (
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

type Discovery struct{}

// discovery.DiscoveryResult
//
// A container for search results returned by a Discovery.
// Discovery results may include SubscriptionLists (collections of Feeds), Feeds, and Items.
type DiscoveryResult struct {
	Subscriptions []*hyperfeed.SubscriptionList
	Feeds         []*hyperfeed.Feed
	Items         []*hyperfeed.Item
}

func (result *DiscoveryResult) TotalCount() int {
	return len(result.Subscriptions) + len(result.Feeds) + len(result.Items)
}
