package hyperfeed

import (
	"context"
	"fmt"
	"strings"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
	"github.com/herdworks/hypergraph-go"
)

// Image represents an image associated with a feed or item.
type Image struct {
	ID      string     `json:"id,omitempty"`
	Href    *href.Href `json:"href,omitempty"`
	Title   string     `json:"title,omitempty"`
	Alt     string     `json:"alt,omitempty"`
	Caption string     `json:"caption,omitempty"`
	Width   int        `json:"width,omitempty"`
	Height  int        `json:"height,omitempty"`
	ItemID  string     `json:"item_id,omitempty"` // back-reference to parent item
	FeedID  string     `json:"feed_id,omitempty"` // back-reference to parent feed
}

// RSSFeedParser.Images()
func ParseImages(item *Item, bytes []byte) (images []*Image) {
	var base href.Href = href.Href(*item.URL)
	var document hypergraph.Document = hypergraph.Document{Href: item.URL.String(), Bytes: bytes}
	var ctx context.Context = context.Background()
	var err error
	if err = document.Scrape(ctx, true, nil); err != nil {
		return images
	}
	for _, i := range document.Images {
		for _, s := range i.Sources {
			var img Image
			if img.Href, err = href.Parse(s.Href); err != nil {
				continue
			}
			img.Href = base.ResolveReference(img.Href.URL())
			img.Title = i.Title
			img.Alt = i.Alt
			img.Caption = i.Caption
			img.Width = s.Width
			img.Height = s.Height
			img.ItemID = item.Identifier()
			img.FeedID = item.FeedID
			if img.Width == 1 || img.Height == 1 {
				continue // skip tracking pixel images
			}
			images = append(images, &img)
		}
	}
	return images
}

func (image *Image) Identifier() string {
	if image == nil {
		return ""
	}
	if image.ID == "" {
		u := normalized.URL(image.Href.URL())
		image.ID = normalized.Identifier(image.ItemID + u.String())
	}
	return image.ID
}

func (Image) DatabaseView() string {
	return "images_view"
}
func (Image) DatabaseViewIdentifier() string {
	return "id"
}
func (Image) DatabaseObject() string {
	return "image"
}
func (Image) DatabaseTable() string {
	return "images"
}
func (Image) DatabaseTableIdentifier() string {
	return "id"
}
func (image *Image) DatabaseInsertColumns() (columns []string) {
	return []string{
		"id",
		"href",
		"title",
		"alt",
		"caption",
		"width",
		"height",
		"item_id",
		"feed_id",
	}
}
func (image *Image) DatabaseInsertValues() (args []any) {
	args = append(args, image.Identifier())
	args = append(args, image.Href.String())
	args = append(args, image.Title)
	args = append(args, strings.TrimSpace(image.Alt))
	args = append(args, strings.TrimSpace(image.Caption))
	args = append(args, image.Width)
	args = append(args, image.Height)
	args = append(args, image.ItemID)
	args = append(args, image.FeedID)
	return args
}

func (Image) DatabaseUpdatesSet() []string {
	return []string{
		"url = excluded.url",
		"title = excluded.title",
		"alt = excluded.alt",
		"caption = excluded.caption",
		"width = excluded.width",
		"height = excluded.height",
	}
}

func (image *Image) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%d|%d|%s",
		image.Href.String(),
		image.Title,
		strings.TrimSpace(image.Alt),
		strings.TrimSpace(image.Caption),
		image.Width,
		image.Height,
		image.ItemID,
	)
}
