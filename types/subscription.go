package hyperfeed

import (
	"encoding/json"
	"net/url"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
)

type SubscriptionList struct {
	ID string `json:"id,omitempty"` // stable HyperFeed SubscriptionList identifier (e.g. hash of URI+owner)
	// URI *url.URL `json:"uri,omitempty"` // URI is the location of the OPML file representing the subscription list.
	Path string `json:"path,omitempty"` // Path is the string representation of SubscriptionList.URI

	// OPML holds the parsed OPML document.
	OPML *OPML `json:"-"`

	// Standard metadata properties
	Title     string    `json:"title,omitempty"`
	Owner     Owner     `json:"owner,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`

	// Feeds contains the list of feeds in the subscription list.
	Feeds []*Feed `json:"feeds,omitempty"`

	// Status holds runtime status information about the subscription list.
	Status SubscriptionStatus `json:"status,omitempty"`
}

type Owner struct {
	// Standard properties
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`
}

type SubscriptionStatus struct {
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
	FeedsSkipped   int       `json:"feeds_skipped,omitempty"`
	ItemsFetched   int       `json:"items_fetched,omitempty"`
	ItemsDuplicate int       `json:"items_duplicate,omitempty"`
	ItemsAdded     int       `json:"items_added,omitempty"`
}

// =========================== SubscriptionList Methods =======================

func (sl *SubscriptionList) Load() (err error) {
	sl.OPML = &OPML{URI: &url.URL{Path: sl.Path}}
	if err = sl.OPML.Load(); err != nil {
		return err
	}
	sl.Title = sl.OPML.Head.Title
	sl.Owner.ID = sl.OPML.Head.OwnerId
	sl.Owner.Name = sl.OPML.Head.OwnerName
	sl.Owner.Email = sl.OPML.Head.OwnerEmail
	if sl.CreatedAt, err = sl.OPML.DateCreated(); err != nil {
		sl.CreatedAt = time.Now()
	}
	if sl.UpdatedAt, err = sl.OPML.DateModified(); err != nil {
		sl.UpdatedAt = sl.CreatedAt
	}
	return nil
}

func (subscription *SubscriptionList) Identifier() string {
	if subscription != nil && subscription.ID != "" {
		return subscription.ID
	}
	subscription.ID = normalized.Identifier(subscription.Path)
	return subscription.ID
}

func (subscription *SubscriptionList) RowTable() string {
	return "subscriptionlists"
}
func (subscription *SubscriptionList) RowIdentifier() string {
	return "id"
}
func (subscription *SubscriptionList) RowColumns() []string {
	return []string{
		"id",
		"uri",
		"title",
		"owner_name",
		"owner_email",
		"created_at",
		"updated_at",
		"status",
	}
}
func (subscription *SubscriptionList) RowArguments() (args []any) {
	args = append(args, subscription.Identifier())
	args = append(args, subscription.Path)
	args = append(args, subscription.Title)
	args = append(args, subscription.Owner.Name)
	args = append(args, subscription.Owner.Email)
	args = append(args, subscription.CreatedAt.UTC().Format(time.RFC3339))
	args = append(args, subscription.UpdatedAt.UTC().Format(time.RFC3339))
	if statusBytes, err := json.Marshal(subscription.Status); err == nil && statusBytes != nil {
		args = append(args, string(statusBytes)) // status
	} else {
		args = append(args, "{}") // emtpy status
	}
	return args
}
func (subscription *SubscriptionList) RowUpdates() [][2]string {
	return [][2]string{
		{"uri", "excluded.uri"},
		{"title", "excluded.title"},
		{"owner_name", "excluded.owner_name"},
		{"owner_email", "excluded.owner_email"},
		{"created_at", "excluded.created_at"},
		{"updated_at", "excluded.updated_at"},
		{"status", "excluded.status"},
	}
}

func (subscription *SubscriptionList) RowTargets() (targets []any) {
	targets = append(targets, &subscription.ID)
	targets = append(targets, &subscription.Path)
	targets = append(targets, &subscription.Title)
	targets = append(targets, &subscription.Owner.Name)
	targets = append(targets, &subscription.Owner.Email)
	targets = append(targets, &subscription.CreatedAt)
	targets = append(targets, &subscription.UpdatedAt)
	targets = append(targets, &subscription.Status)
	return targets
}
