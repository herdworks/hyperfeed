package hyperfeed

import (
	"net/url"
	"time"

	opml "github.com/herdworks/hypertemplates-go/opml"
)

type SubscriptionList struct {
	// URI is the location of the OPML file representing the subscription list.
	URI *url.URL `json:"uri,omitempty"`

	// OPML holds the parsed OPML document.
	OPML *opml.OPML `json:"-"`

	// Standard metadata properties
	Title     string    `json:"title,omitempty"`
	Owner     Owner     `json:"owner,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`

	// Feeds contains the list of feeds in the subscription list.
	Feeds []*Feed `json:"feeds,omitempty"`
}

type SubscriptionLists []*SubscriptionList

func (sl *SubscriptionList) Load() (err error) {
	sl.OPML = &opml.OPML{URI: sl.URI}
	if err = sl.OPML.Load(); err != nil {
		return err
	}
	sl.Title = sl.OPML.Head.Title
	if sl.CreatedAt, err = sl.OPML.DateCreated(); err != nil {
		return err
	}
	if sl.UpdatedAt, err = sl.OPML.DateModified(); err != nil {
		return err
	}
	return nil
}

type Owner struct {
	// Standard properties
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`
}
