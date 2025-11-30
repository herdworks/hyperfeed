package hyperfeed

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
)

// ============================ Package Types =================================

// Item represents an entry within a feed.
type Item struct {
	ID  string   `json:"id,omitempty"`
	URL *url.URL `json:"url,omitempty"` // URL is the canonical link for the item (permalink).

	PublishedAt time.Time `json:"published_at,omitempty"` // source item created timestamp
	ModifiedAt  time.Time `json:"modified_at,omitempty"`  // source item last modified timestamp
	AddedAt     time.Time `json:"added_at,omitempty"`     // when we first saw the item
	UpdatedAt   time.Time `json:"updated_at,omitempty"`   // when we last updated the item in our database
	FeedAt      time.Time `json:"feed_at,omitempty"`      // default feed order timestamp

	Title       string       `json:"title,omitempty"`
	Bylines     []Author     `json:"bylines,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Content     string       `json:"content,omitempty"`
	Images      []*Image     `json:"images,omitempty"` // associated images (e.g. cover images, thumbnails)
	Links       []*url.URL   `json:"links,omitempty"`
	Categories  []string     `json:"categories,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"` // RSS enclosures, JSONFeed attachments

	// Comments is a URL pointing to a comments page for the item.
	Comments *url.URL `json:"comments,omitempty"`

	// Metadata holds non-standard/extension Item properties
	Metadata Metadata `json:"metadata,omitempty"`

	FeedID string `json:"feed_id,omitempty"` // back-reference to parent feed
	Feed   *Feed  `json:"-"`                 // parent feed (not serialized)
}

// ============================ Item Methods ==================================

func (i *Item) Ready() bool {
	if i == nil {
		return false
	}
	if i.URL == nil {
		return false
	}
	return true
}

func (item *Item) Identifier() string {
	if item == nil {
		return ""
	}
	if item.ID == "" {
		u := normalized.URL(item.URL)
		item.ID = normalized.Identifier(item.FeedID + u.String())
	}
	return item.ID
}

// TimelineAt computes the HyperFeed default "chronological" timeline timestamp for an item
// 
// TimelineAt uses the following logic:
// - If timelinePosition is zero, return the item's actual timestamp (PublishedAt or ModifiedAt).
// - If the item's actual timestamp is after timelinePosition and before now, return the actual timestamp.
// - If the item's actual timestamp is after timelinePosition and also after now, return AddedAt.
// - If AddedAt is after timelinePosition, return AddedAt.
// - Otherwise, return timelinePosition + 1 minute to nudge the item into the timeline.
func (item *Item) TimelineAt(timelinePosition time.Time) (ts time.Time) {
	ts = Earliest(item.PublishedAt, item.ModifiedAt) // actual item timestamp
	if timelinePosition.IsZero() && ts.Before(time.Now()) {
		return ts.UTC() // new timeline, use actual item timestamp
	}
	if ts.After(timelinePosition) && ts.Before(time.Now()) {
		return ts.UTC() // item is between timeline position and "now", use actual item timestamp
	}
	if ts.After(timelinePosition) && ts.After(time.Now()) {
		return item.AddedAt.UTC() // item has invalid future added timestamp, use added timestamp
	}
	if item.AddedAt.After(timelinePosition) {
		return item.AddedAt.UTC() // item is behind timeline but was added after timeline position
	}
	// the timeline is scrolled and the item is older than the timeline position
	return timelinePosition.Add(1 * time.Minute) // nudge item into timeline
}

func (Item) DatabaseView() string {
	return "items_view"
}
func (Item) DatabaseViewIdentifier() string {
	return "id"
}
func (Item) DatabaseObject() string {
	return "item"
}
func (Item) DatabaseTable() string {
	return "items"
}
func (Item) DatabaseTableIdentifier() string {
	return "id"
}
func (Item) DatabaseInsertColumns() (columns []string) {
	return []string{
		"id",
		"url",
		"title",
		"summary",
		"content",
		"published_at",
		"modified_at",
		"added_at",
		"updated_at",
		"feed_at",
		"feed_id",
	}
}
func (item *Item) DatabaseInsertValues() (args []any) {
	args = append(args, item.Identifier())
	args = append(args, item.URL.String())
	args = append(args, item.Title)
	args = append(args, strings.TrimSpace(item.Summary))
	args = append(args, strings.TrimSpace(item.Content))
	args = append(args, item.PublishedAt.UTC().Format(time.RFC3339))
	args = append(args, item.ModifiedAt.UTC().Format(time.RFC3339))
	args = append(args, item.AddedAt.UTC().Format(time.RFC3339))
	args = append(args, item.UpdatedAt.UTC().Format(time.RFC3339))
	args = append(args, item.FeedAt.UTC().Format(time.RFC3339))
	args = append(args, item.FeedID)
	return args
}
func (item *Item) DatabaseUpdatesSet() []string {
	return []string{
		"url = excluded.url",
		"title = excluded.title",
		"summary = excluded.summary",
		"content = excluded.content",
		"modified_at = excluded.modified_at",
		"updated_at = excluded.updated_at",
	}
}

func (item *Item) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s",
		item.URL.String(),
		item.Title,
		strings.TrimSpace(item.Summary),
		strings.TrimSpace(item.Content),
		item.PublishedAt.UTC().Format(time.RFC3339),
		item.ModifiedAt.UTC().Format(time.RFC3339),
		item.FeedID,
	)
}
