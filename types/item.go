package hyperfeed

import (
	"net/url"
	"strings"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
)

// ============================ Package Types =================================

// Item represents an entry within a feed.
type Item struct {
	// URL is the canonical link for the item (permalink).
	URL *url.URL `json:"url,omitempty"`

	// GUID is a stable identifier provided by the source feed when available.
	GUID string `json:"guid,omitempty"`

	// ID is the stable HyperFeed Item identifier (e.g., a UUID or hash).
	ID string `json:"id,omitempty"`

	Title   string   `json:"title,omitempty"`
	Authors []Author `json:"authors,omitempty"`

	Published time.Time `json:"published,omitempty"` // created timestamp
	Updated   time.Time `json:"updated,omitempty"`   // modified timestamp
	Added     time.Time `json:"added,omitempty"`     // when we first saw the item

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

func (item *Item) Identifier() any {
	if item.ID == "" {
		u := normalized.URL(item.URL)
		item.ID = normalized.Identifier(u.String())
	}
	return item.ID
}

func (item *Item) RowTable() string {
	return "items"
}
func (item *Item) RowIdentifier() string {
	return "id"
}
func (item *Item) RowColumns() (columns []string) {
	return []string{"id", "guid", "url", "title", "summary", "content", "published", "updated", "added"}
}
func (item *Item) RowArguments() (args []any) {
	args = append(args, item.Identifier())
	args = append(args, item.GUID)
	if item.URL != nil {
		args = append(args, item.URL.String())
	} else {
		args = append(args, "")
	}
	args = append(args, item.Title)
	args = append(args, strings.TrimSpace(item.Summary))
	args = append(args, strings.TrimSpace(item.Content))
	args = append(args, item.Published.UTC().Format(time.RFC3339))
	args = append(args, item.Updated.UTC().Format(time.RFC3339))
	args = append(args, item.Added.UTC().Format(time.RFC3339))
	return args
}
func (item *Item) RowUpdates() [][2]string {
	return [][2]string{
		{"guid", "excluded.guid"},
		{"url", "excluded.url"},
		{"title", "excluded.title"},
		{"summary", "excluded.summary"},
		{"content", "excluded.content"},
		{"published", "excluded.published"},
		{"updated", "excluded.updated"},
		{"added", "excluded.added"},
	}
}
