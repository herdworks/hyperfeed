package hyperfeed

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// Attachment represents a generic media attachment associated with an item.
// It serves as a lowest-common-denominator across RSS enclosures, Atom
// enclosures (link rel="enclosure"), and JSON Feed attachments.
//
// Fields map as follows (where available):
// - URL/href -> URL
// - title (Atom link@title or other extensions) -> Title
// - length/size_in_bytes -> Size
// - type/mime_type -> Mimetype
// - duration_in_seconds (JSON Feed, podcast extensions) -> Duration
// - labels/tags (non-standard, commonly present in extensions) -> Labels
type Attachment struct {
	*DatabaseRecord

	ID       string     `json:"id,omitempty"` // stable HyperFeed Attachment identifier (e.g. hash of URL+title)
	URL      *href.Href `json:"url,omitempty"`
	Title    string     `json:"title,omitempty"`
	Size     int64      `json:"size,omitempty"`     // bytes
	Duration int64      `json:"duration,omitempty"` // seconds
	Mimetype string     `json:"mimetype,omitempty"` // MIME type
	Labels   []string   `json:"labels,omitempty"`

	ItemID    string `json:"-"` // back-reference to parent item
	FeedID    string `json:"-"` // back-reference to parent feed
	WebsiteID string `json:"-"` // back-reference to parent website
}

// ============================ Attachment Methods ============================

func (attachment *Attachment) Identifier() string {
	if attachment == nil {
		return ""
	}
	if attachment.ID == "" {
		var id *url.URL = normalized.URL(attachment.URL.URL())
		var query url.Values = id.Query()
		if id == nil {
			id = &url.URL{}
		}
		query.Set("_size", fmt.Sprintf("%d", attachment.Size))
		query.Set("_duration", fmt.Sprintf("%d", attachment.Duration))
		query.Set("_mimetype", attachment.Mimetype)
		id.RawQuery = query.Encode()
		attachment.ID = normalized.Identifier(normalized.URL(id).String())
	}
	return attachment.ID
}
func (*Attachment) DatabaseObject() string {
	return "attachment"
}
func (*Attachment) DatabaseTable() string {
	return "attachments"
}
func (*Attachment) DatabaseView() (name string) {
	return "attachments_view"
}
func (*Attachment) DatabaseInsertColumns() (columns []string) {
	return []string{
		"id",
		"url",
		"title",
		"size",
		"duration",
		"mimetype",
		"labels",
		"item_id",
		"feed_id",
		"website_id",
	}
}
func (attachment *Attachment) DatabaseInsertValues() (values []any) {
	values = append(values, sql.Named("id", attachment.Identifier()))
	values = append(values, sql.Named("url", attachment.URL.String()))
	values = append(values, sql.Named("title", attachment.Title))
	values = append(values, sql.Named("size", attachment.Size))
	values = append(values, sql.Named("duration", attachment.Duration))
	values = append(values, sql.Named("mimetype", attachment.Mimetype))
	values = append(values, sql.Named("labels", strings.Join(attachment.Labels, ", ")))
	values = append(values, sql.Named("item_id", attachment.ItemID))
	values = append(values, sql.Named("feed_id", attachment.FeedID))
	values = append(values, sql.Named("website_id", attachment.WebsiteID))
	return values
}
func (attachment *Attachment) Hash() string {
	return fmt.Sprintf(
		"%s|%d|%d|%s|%s",
		attachment.URL.String(),
		attachment.Size,
		attachment.Duration,
		attachment.Mimetype,
		strings.Join(attachment.Labels, ","),
	)
}
