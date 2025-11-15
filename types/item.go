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

// Image represents an image associated with a feed or item.
type Image struct {
	URL    *url.URL `json:"url,omitempty"`
	Title  string   `json:"title,omitempty"`
	Width  int      `json:"width,omitempty"`
	Height int      `json:"height,omitempty"`
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
	args = append(args, item.FeedID)
	return args
}
func (item *Item) DatabaseUpdatesSet() []string {
	return []string{
		"url = excluded.url",
		"title = excluded.title",
		"summary = excluded.summary",
		"content = excluded.content",
		"published_at = excluded.published_at",
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
