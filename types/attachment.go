package hyperfeed

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"slices"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go/types"
	"golang.org/x/net/html"
)

type Attachment struct {
	*DatabaseRecord

	ID              string     `json:"id,omitempty"`   // stable HyperFeed Attachment identifier (e.g. hash of URL+title)
	Href            *href.Href `json:"href,omitempty"` // default URL of the attachment
	Sources         []Source   `json:"sources,omitempty"`
	Poster          *href.Href `json:"poster,omitempty"`   // poster image for audio/video attachments
	Mimetype        string     `json:"mimetype,omitempty"` // MIME type
	Title           string     `json:"title,omitempty"`
	Description     string     `json:"description,omitempty"`
	Enclosure       bool       `json:"enclosure,omitempty"`
	Download        bool       `json:"download,omitempty"`
	Rank            int        `json:"rank,omitempty"`
	Size            int64      `json:"size,omitempty"`             // bytes
	Duration        int64      `json:"duration,omitempty"`         // seconds
	DurationWatched int64      `json:"duration_watched,omitempty"` // seconds watched
	Height          int        `json:"height,omitempty"`
	Width           int        `json:"width,omitempty"`
}

type Source struct {
	Mimetype  string     `json:"mimetype"`
	Href      *href.Href `json:"href"`
	Width     int        `json:"width,omitempty"`
	Height    int        `json:"height,omitempty"`
	Container string     `json:"container,omitempty"`
}

// ============================ Attachment Methods ============================

func (attachment *Attachment) Identifier() string {
	if attachment == nil {
		return normalized.Identifier("") // the unknown identifier
	}
	if attachment.ID == "" {
		id := normalized.URL(attachment.Href.URL())
		attachment.ID = normalized.Identifier(id.String())
	}
	return attachment.ID
}

// ParseItemLinkAttachments() generates link attachments from a []*url.URL slice
func ParseItemLinkAttachments(item *Item, urls []*url.URL, w io.Writer) (attachments []Attachment) {
	for _, u := range urls {
		if base, err := href.Parse(item.URL); err != nil || base == nil || base.SameOrigin(u) {
			continue // skip invalid or same-origin links
		}
		var attachment Attachment = Attachment{Sources: []Source{}, Mimetype: "text/x-url"}
		attachment.Href, _ = href.Parse(u)
		attachment.Enclosure = false
		attachment.Rank = len(attachments) + 1
		attachments = append(attachments, attachment)
	}
	return attachments
}

// ParseItemMediaAttachments() extracts media from an item's HTML content
func ParseItemMediaAttachments(item *Item, htmlBytes []byte, w io.Writer) (attachments []Attachment) {
	var document hypergraph.Document = hypergraph.Document{}
	var ctx context.Context = context.Background()
	var err error
	if item.URL != nil {
		document.Href = item.URL.String()
	} else {
		document.Href = "about:blank" // TODO: setting document.Href for hypergraph's Document.Ready() 🤦 until we finish the hypergraph refactor
	}
	if document.HTML, err = html.Parse(bytes.NewReader(htmlBytes)); err == nil {
		document.Root = document.HTML
	} else {
		return attachments
	}
	if err = document.Scrape(ctx, true, nil, 5, w); err != nil {
		return attachments
	}
	for _, media := range document.Media {
		var attachment Attachment = Attachment{
			Title:       media.Title,
			Description: media.Description,
			Rank:        media.Index,
			Download:    media.Download,
		}
		for _, s := range media.Sources {
			var src Source
			if src.Href, err = href.Parse(s.Href); err != nil {
				continue
			}
			src.Mimetype = s.Type
			src.Width = s.Width
			src.Height = s.Height
			if src.Width == 1 || src.Height == 1 {
				continue // skip tracking pixel images
			}
			attachment.Sources = append(attachment.Sources, src)
		}
		if len(attachment.Sources) == 0 {
			continue // skip any attachment with no valid sources
		}
		slices.SortFunc(attachment.Sources, func(a, b Source) int {
			return cmp.Compare(b.Width, a.Width) // sort sources by width (largest to smallest)
		})
		attachment.Href = attachment.Sources[0].Href // primary source is the largest
		attachment.Mimetype = attachment.Sources[0].Mimetype
		attachment.Width = attachment.Sources[0].Width
		attachment.Height = attachment.Sources[0].Height
		attachment.Sources = attachment.Sources[1:] // remove primary source from sources list
		attachments = append(attachments, attachment)
	}
	return attachments
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
		"href",
		"sources",
		"poster",
		"mimetype",
		"title",
		"description",
		"enclosure",
		"download",
		"rank",
		"size",
		"duration",
		"duration_watched",
		"height",
		"width",
	}
}
func (attachment *Attachment) DatabaseInsertValues() (values []any) {
	values = append(values, sql.Named("id", attachment.Identifier()))
	values = append(values, sql.Named("href", attachment.Href.String()))
	if jsonBytes, err := json.Marshal(attachment.Sources); err == nil {
		values = append(values, sql.Named("sources", string(jsonBytes)))
	} else {
		values = append(values, sql.Named("sources", "[]"))
	}
	if attachment.Poster != nil {
		values = append(values, sql.Named("poster", attachment.Poster.String()))
	} else {
		values = append(values, sql.Named("poster", nil))
	}
	values = append(values, sql.Named("mimetype", attachment.Mimetype))
	values = append(values, sql.Named("title", attachment.Title))
	values = append(values, sql.Named("description", attachment.Description))
	values = append(values, sql.Named("enclosure", attachment.Enclosure))
	values = append(values, sql.Named("download", attachment.Download))
	values = append(values, sql.Named("rank", attachment.Rank))
	values = append(values, sql.Named("size", attachment.Size))
	values = append(values, sql.Named("duration", attachment.Duration))
	values = append(values, sql.Named("duration_watched", attachment.DurationWatched))
	values = append(values, sql.Named("height", attachment.Height))
	values = append(values, sql.Named("width", attachment.Width))
	return values
}
func (attachment *Attachment) Hash() string {
	return fmt.Sprintf(
		"%s|%v|%d|%d|%s",
		attachment.Href.String(),
		attachment.Sources,
		attachment.Size,
		attachment.Duration,
		attachment.Mimetype,
	)
}
