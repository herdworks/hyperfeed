package hyperfeed

import (
    "net/url"

    opml "github.com/herdworks/hypertemplates-go/opml"
)

type SubscriptionList struct {
    // URI is the location of the OPML file representing the subscription list.
    URI *url.URL `json:"uri,omitempty"`

    // Title is a human-friendly name for the subscription list.
    Title string `json:"title,omitempty"`

    // OPML holds the parsed OPML document.
    OPML *opml.OPML `json:"-"`

    // Metadata holds extension data for other packages (subscriber, scheduler, etc.).
    Metadata Metadata `json:"metadata,omitempty"`
}
type SubscriptionLists []*SubscriptionList
