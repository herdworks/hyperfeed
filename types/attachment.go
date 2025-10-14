package hyperfeed

import (
	"fmt"
	"net/url"
	"strings"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
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
	ID       string   `json:"id,omitempty"` // stable HyperFeed Attachment identifier (e.g. hash of URL+title)
	URL      *url.URL `json:"url,omitempty"`
	Title    string   `json:"title,omitempty"`
	Size     int64    `json:"size,omitempty"`     // bytes
	Duration int64    `json:"duration,omitempty"` // seconds
	Mimetype string   `json:"mimetype,omitempty"` // MIME type
	Labels   []string `json:"labels,omitempty"`

	ItemID string `json:"-"` // back-reference to parent item
}

// ============================ Attachment Methods ============================

func (attachment *Attachment) Identifier() string {
	if attachment != nil && attachment.ID != "" {
		return attachment.ID
	}
	var id *url.URL = normalized.URL(attachment.URL)
	var query url.Values = id.Query()
	if id == nil {
		id = &url.URL{}
	}
	query.Set("_size", fmt.Sprintf("%d", attachment.Size))
	query.Set("_duration", fmt.Sprintf("%d", attachment.Duration))
	query.Set("_mimetype", attachment.Mimetype)
	id.RawQuery = query.Encode()
	return normalized.Identifier(normalized.URL(id).String())
}
func (attachment *Attachment) RowTable() (name string) {
	return "attachments"
}
func (attachment *Attachment) RowIdentifier() string {
	return "id"
}
func (attachment *Attachment) RowColumns() (columns []string) {
	return []string{
		"id",
		"url",
		"title",
		"size",
		"duration",
		"mimetype",
		"labels",
		"item_id",
	}
}
func (attachment *Attachment) RowArguments() []any {
	if attachment.ItemID == "" {
		fmt.Printf("WARNING: Attachment missing ItemID back-reference: %+v\n", attachment)
	}
	return []any{
		attachment.Identifier(),
		attachment.URL.String(),
		attachment.Title,
		attachment.Size,
		attachment.Duration,
		attachment.Mimetype,
		strings.Join(attachment.Labels, ", "),
		attachment.ItemID,
	}
}
func (attachment *Attachment) RowUpdates() [][2]string {
	return [][2]string{
		{"title", "excluded.title"},
		{"size", "excluded.size"},
		{"duration", "excluded.duration"},
		{"mimetype", "excluded.mimetype"},
		{"labels", "excluded.labels"},
		{"item_id", "excluded.item_id"},
	}
}
func (attachment *Attachment) RowTargets() (targets []any) {
	targets = append(targets, &attachment.ID)
	targets = append(targets, &attachment.URL)
	targets = append(targets, &attachment.Title)
	targets = append(targets, &attachment.Size)
	targets = append(targets, &attachment.Duration)
	targets = append(targets, &attachment.Mimetype)
	targets = append(targets, &attachment.Labels)
	return targets
}
