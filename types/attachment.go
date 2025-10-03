package hyperfeed

import (
	"fmt"
	"net/url"
	"strings"
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
	URL      *url.URL `json:"url,omitempty"`
	Title    string   `json:"title,omitempty"`
	Size     int64    `json:"size,omitempty"`     // bytes
	Duration int64    `json:"duration,omitempty"` // seconds
	Mimetype string   `json:"mimetype,omitempty"` // MIME type
	Labels   []string `json:"labels,omitempty"`

	ItemID string `json:"-"` // back-reference to parent item
}

func (attachment *Attachment) RowTable() (name string) {
	return "attachments"
}
func (attachment *Attachment) RowColumns() (columns []string) {
	return []string{
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
