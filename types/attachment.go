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

	ID          string     `json:"id,omitempty"`   // stable HyperFeed Attachment identifier (e.g. hash of URL+title)
	Href        *href.Href `json:"href,omitempty"` // default URL of the attachment
	Sources     []Source   `json:"sources,omitempty"`
	Poster      *href.Href `json:"poster,omitempty"`   // poster image for audio/video attachments
	Mimetype    string     `json:"mimetype,omitempty"` // MIME type
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Enclosure   bool       `json:"enclosure,omitempty"`
	Download    bool       `json:"download,omitempty"`
	Rank        int        `json:"rank,omitempty"`
	Size        int64      `json:"size,omitempty"`     // bytes
	Duration    int64      `json:"duration,omitempty"` // seconds
	Height      int        `json:"height,omitempty"`
	Width       int        `json:"width,omitempty"`
	ItemID      string     `json:"-"` // back-reference to parent item
	FeedID      string     `json:"-"` // back-reference to parent feed
	WebsiteID   string     `json:"-"` // back-reference to parent website
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
		return ""
	}
	if attachment.ID == "" {
		var id *url.URL = normalized.URL(attachment.Href.URL())
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

// ParseItemMedia() extracts media from an item's HTML content
func ParseItemMedia(item *Item, htmlBytes []byte, w io.Writer) (attachments []Attachment) {
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
			ItemID:      item.Identifier(),
			FeedID:      item.Feed.Identifier(),
			WebsiteID:   item.Feed.Website.Identifier(),
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
		"height",
		"width",
		"item_id",
		"feed_id",
		"website_id",
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
	values = append(values, sql.Named("height", attachment.Height))
	values = append(values, sql.Named("width", attachment.Width))
	values = append(values, sql.Named("item_id", attachment.ItemID))
	values = append(values, sql.Named("feed_id", attachment.FeedID))
	values = append(values, sql.Named("website_id", attachment.WebsiteID))
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
