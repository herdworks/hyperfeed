package hyperfeed

import "net/url"

// Attachment represents a generic media attachment associated with an item.
// It serves as a lowest-common-denominator across RSS enclosures, Atom
// enclosures (link rel="enclosure"), and JSON Feed attachments.
//
// Fields map as follows (where available):
// - URL/href -> URL
// - length/size_in_bytes -> Size
// - type/mime_type -> Mimetype
// - duration_in_seconds (JSON Feed, podcast extensions) -> Duration
// - title (Atom link@title or other extensions) -> Title
// - labels/tags (non-standard, commonly present in extensions) -> Labels
type Attachment struct {
	Title    string   `json:"title,omitempty"`
	URL      *url.URL `json:"url,omitempty"`
	Size     int64    `json:"size,omitempty"`     // bytes
	Duration int64    `json:"duration,omitempty"` // seconds
	Mimetype string   `json:"mimetype,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}
