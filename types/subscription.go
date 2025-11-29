package hyperfeed

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	"github.com/herdworks/hyperfeed-go/types/opml"
)

type SubscriptionList struct {
	ID string `json:"id,omitempty"` // stable HyperFeed SubscriptionList identifier (e.g. hash of URI+owner)
	// URI *url.URL `json:"uri,omitempty"` // URI is the location of the OPML file representing the subscription list.
	Path string `json:"path,omitempty"` // Path is the string representation of SubscriptionList.URI

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

	// Status holds runtime status information about the subscription list.
	Status *SubscriptionStatus `json:"status,omitempty"`
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
	Duration       time.Duration `json:"duration,omitempty"`
	UpdatedAt      time.Time     `json:"updated_at,omitempty"`
	FeedsSkipped   Counter       `json:"feeds_skipped,omitempty"`
	FeedsFailed    Counter       `json:"feeds_failed,omitempty"`
	FeedsFetched   Counter       `json:"feeds_fetched,omitempty"`
	ItemsFetched   Counter       `json:"items_fetched,omitempty"`
	ItemsDuplicate Counter       `json:"items_duplicate,omitempty"`
	ItemsAdded     Counter       `json:"items_added,omitempty"`
	ItemsUpdated   Counter       `json:"items_updated,omitempty"`
}

// =========================== SubscriptionList Methods =======================

func (sl *SubscriptionList) Load() (err error) {
	sl.OPML = &opml.OPML{URI: &url.URL{Path: sl.Path}}
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
func (SubscriptionList) DatabaseView() string {
	return "subscriptionlists_view"
}
func (SubscriptionList) DatabaseViewIdentifier() string {
	return "id"
}
func (SubscriptionList) DatabaseObject() string {
	return "subscriptionlist"
}
func (SubscriptionList) DatabaseTable() string {
	return "subscriptionlists"
}
func (SubscriptionList) DatabaseTableIdentifier() string {
	return "id"
}
func (SubscriptionList) DatabaseInsertColumns() []string {
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
func (subscription *SubscriptionList) DatabaseInsertValues() (args []any) {
	statusBytes, _ := json.Marshal(subscription.Status)
	args = append(args, subscription.Identifier())
	args = append(args, subscription.Path)
	args = append(args, subscription.Title)
	args = append(args, subscription.Owner.Name)
	args = append(args, subscription.Owner.Email)
	args = append(args, subscription.CreatedAt.UTC().Format(time.RFC3339))
	args = append(args, subscription.UpdatedAt.UTC().Format(time.RFC3339))
	args = append(args, string(statusBytes)) // status
	return args
}
func (subscription *SubscriptionList) DatabaseUpdatesSet() []string {
	return []string{
		"uri = excluded.uri",
		"title = excluded.title",
		"owner_name = excluded.owner_name",
		"owner_email = excluded.owner_email",
		"created_at = excluded.created_at",
		"updated_at = excluded.updated_at",
		"status = excluded.status",
	}
}

func (subscription *SubscriptionList) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s",
		subscription.Identifier(),
		subscription.Path,
		subscription.Title,
		subscription.Owner.Name,
		subscription.Owner.Email,
		subscription.CreatedAt.UTC().Format(time.RFC3339),
	)
}

// ======================= SubscriptionStatus Methods =========================

func (status *SubscriptionStatus) Print() string {
	var report string = "\n"
	report += fmt.Sprintf("Feeds Skipped:   %6d\n", status.FeedsSkipped.Count)
	report += fmt.Sprintf("Feeds Failed:    %6d\n", status.FeedsFailed.Count)
	report += fmt.Sprintf("Feeds Fetched:   %6d\n", status.FeedsFetched.Count)
	report += fmt.Sprintf("Items Fetched:   %6d\n", status.ItemsFetched.Count)
	report += fmt.Sprintf("Items Duplicate: %6d\n", status.ItemsDuplicate.Count)
	report += fmt.Sprintf("Items Updated:   %6d\n", status.ItemsUpdated.Count)
	report += fmt.Sprintf("Items Added:     %6d\n", status.ItemsAdded.Count)
	report += "-----------------------\n"
	report += fmt.Sprintf("TOTAL:           %6v\n", status.Duration.Truncate(time.Millisecond))
	return report
}
