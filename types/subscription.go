package hyperfeed

import (
	"net/url"

	opml "github.com/herdworks/hypertemplates-go/opml"
)

type SubscriptionList struct {
	URI      *url.URL
	OPML     *opml.OPML
	Metadata Metadata
}
type SubscriptionLists []*SubscriptionList
