package hyperfeed

import (
	"testing"
)

func TestSubscriptionListLoad(t *testing.T) {
	var subscriptionList SubscriptionList
	var err error
	subscriptionList = SubscriptionList{Path: "testdata/2025-09-09/nnw_default_subscriptions.opml"}
	if err = subscriptionList.Load(); err != nil {
		t.Fatalf("Failed to load subscription list: %v", err)
	}
	if len(subscriptionList.OPML.Body.Outlines) != 14 {
		t.Fatalf("Expected 14 outlines; got %d", len(subscriptionList.OPML.Body.Outlines))
	}
}
