package hyperfeed

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go"
)

// Image represents an image associated with a feed or item.
type Image struct {
	ID      string       `json:"id,omitempty"`
	Href    *href.Href   `json:"href,omitempty"`
	Sources []ImageSrc   `json:"sources,omitempty"`
	Title   string       `json:"title,omitempty"`
	Alt     string       `json:"alt,omitempty"`
	Caption string       `json:"caption,omitempty"`
	Width   int          `json:"width,omitempty"`
	Height  int          `json:"height,omitempty"`
	ItemID  string       `json:"item_id,omitempty"` // back-reference to parent item
	FeedID  string       `json:"feed_id,omitempty"` // back-reference to parent feed
}
type ImageSrc struct {
	Href  *href.Href `json:"href,omitempty"`
	Width int        `json:"width,omitempty"`
	Height int       `json:"height,omitempty"`
}

// ParseImages() extracts images from an item's HTML content
func ParseImages(item *Item, bytes []byte) (images []*Image) {
	var base href.Href = href.Href(*item.URL)
	var document hypergraph.Document = hypergraph.Document{Href: item.URL.String(), Bytes: bytes}
	var ctx context.Context = context.Background()
	var err error
	if err = document.Scrape(ctx, true, nil); err != nil {
		return images
	}
	for _, i := range document.Images {
		var img Image = Image{
			Title: i.Title,
			Alt: i.Alt,
			Caption: i.Caption,
			ItemID: item.Identifier(),
			FeedID: item.FeedID,
		}
		for _, s := range i.Sources {
			var src ImageSrc
			if src.Href, err = href.Parse(s.Href); err != nil {
				continue
			}
			src.Href = base.ResolveReference(src.Href.URL())
			src.Width = s.Width
			src.Height = s.Height
			if src.Width == 1 || src.Height == 1 {
				continue // skip tracking pixel images
			}
			img.Sources = append(img.Sources, src)
		}
		if len(img.Sources) == 0 {
			continue // skip images with no valid sources
		}
		slices.SortFunc(img.Sources, func(a, b ImageSrc) int {
			return cmp.Compare(b.Width, a.Width) // sort sources by width (largest to smallest)
		})
		img.Href = img.Sources[0].Href // primary source is the largest
		img.Width = img.Sources[0].Width
		img.Height = img.Sources[0].Height
		img.Sources = img.Sources[1:] // remove primary source from sources list
		images = append(images, &img)
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
		"sources",
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
	if jsonBytes, err := json.Marshal(image.Sources); err == nil {
		args = append(args, string(jsonBytes))
	} else {
		args = append(args, "[]")
	}
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
		"href = excluded.href",
		"sources = excluded.sources",
		"title = excluded.title",
		"alt = excluded.alt",
		"caption = excluded.caption",
		"width = excluded.width",
		"height = excluded.height",
	}
}

func (image *Image) Hash() string {
	return fmt.Sprintf(
		"%s|%v|%s|%s|%s|%d|%d|%s",
		image.Href.String(),
		image.Sources,
		image.Title,
		strings.TrimSpace(image.Alt),
		strings.TrimSpace(image.Caption),
		image.Width,
		image.Height,
		image.ItemID,
	)
}
