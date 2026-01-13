package hyperfeed

import (
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// ============================ Package Types =================================

// Item represents an entry within a feed.
type Item struct {
	*DatabaseRecord

	ID  string   `json:"id,omitempty"`
	URL *url.URL `json:"url,omitempty"` // URL is the canonical link for the item (permalink).

	ExternalHrefs []string `json:"external_hrefs,omitempty"` // ExternalHrefs are other URLs associated with the item.

	PublishedAt time.Time `json:"published_at,omitempty"` // source item created timestamp
	ModifiedAt  time.Time `json:"modified_at,omitempty"`  // source item last modified timestamp
	AddedAt     time.Time `json:"added_at,omitempty"`     // when we first saw the item
	UpdatedAt   time.Time `json:"updated_at,omitempty"`   // when we last updated the item in our database
	FeedAt      time.Time `json:"feed_at,omitempty"`      // default feed order timestamp

	Title       string       `json:"title,omitempty"`
	Bylines     []Byline     `json:"bylines,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Content     string       `json:"content,omitempty"`
	Images      []*Image     `json:"images,omitempty"` // associated images (e.g. cover images, thumbnails)
	Links       []*HyperLink `json:"links,omitempty"`
	Categories  []string     `json:"categories,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"` // RSS enclosures, JSONFeed attachments

	// Comments is a URL pointing to a comments page for the item.
	Comments *url.URL `json:"comments,omitempty"`

	// Metadata holds non-standard/extension Item properties
	Metadata Metadata `json:"metadata,omitempty"`

	// GUID is the original feed-provided unique identifier for the item
	// GUID is only used as Identifier() material
	GUID string `json:"-"`

	// Feed is the parent feed this item belongs to
	Feed *Feed `json:"-"` // parent feed (not serialized)
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
		item.ID = normalized.Identifier(fmt.Sprintf("%s/%s/%s", item.Feed.Website.Identifier(), u, item.GUID))
	}
	return item.ID
}

// ResolveRelativeURLs resolves any relative URLs within the item to absolute URLs
func (item *Item) ResolveRelativeURLs(w io.Writer) {
	var base href.Href
	if item.URL != nil && item.URL.IsAbs() {
		base = href.Href(*item.URL)
	} else if item.Feed != nil && item.Feed.URL != nil && item.Feed.URL.IsAbs() {
		base = *item.Feed.URL
	} else {
		fmt.Fprintf(w, "\n\ncannot resolve relative URLs for item %q: no base URL\n\n", item.Identifier())
		return
	}
	if item.URL != nil {
		item.URL = href.ResolveHref(base, item.URL, w).URL()
	}
	for i, h := range item.ExternalHrefs {
		item.ExternalHrefs[i] = href.ResolveHrefString(base, h, w)
	}
	for i, b := range item.Bylines {
		if b.Href != nil {
			b.Href = href.ResolveHref(base, b.Href, w)
			item.Bylines[i] = b
		}
		if b.Favicon != nil {
			b.Favicon = href.ResolveHref(base, b.Favicon, w)
			item.Bylines[i] = b
		}
	}
	for i, img := range item.Images {
		if img.Href != nil {
			img.Href = href.ResolveHref(base, img.Href, w)
		}
		for j, src := range img.Sources {
			if src.Href != nil {
				src.Href = href.ResolveHref(base, src.Href, w)
				img.Sources[j] = src
			}
		}
		item.Images[i] = img
	}
	for i, a := range item.Attachments {
		if a.URL != nil {
			a.URL = href.ResolveHref(base, a.URL, w)
			item.Attachments[i] = a
		}
	}
}

// TimelineAt computes the HyperFeed default "chronological" timeline timestamp for an item
//
// TimelineAt uses the following logic:
// - If timelinePosition is zero, return the item's actual timestamp (PublishedAt or ModifiedAt).
// - If the item's actual timestamp is after timelinePosition and before now, return the actual timestamp.
// - If the item's actual timestamp is after timelinePosition and also after now, return AddedAt.
// - If AddedAt is after timelinePosition, return AddedAt.
// - Otherwise, return timelinePosition + 1 minute to nudge the item into the timeline.
func (item *Item) TimelineAt(timelinePosition time.Time) (ts time.Time) {
	ts = Earliest(item.PublishedAt, item.ModifiedAt) // actual item timestamp
	if timelinePosition.IsZero() && ts.Before(time.Now()) {
		return ts.UTC() // new timeline, use actual item timestamp
	}
	if ts.After(timelinePosition) && ts.Before(time.Now()) {
		return ts.UTC() // item is between timeline position and "now", use actual item timestamp
	}
	if ts.After(timelinePosition) && ts.After(time.Now()) {
		return item.AddedAt.UTC() // item has invalid future added timestamp, use added timestamp
	}
	if item.AddedAt.After(timelinePosition) {
		return item.AddedAt.UTC() // item is behind timeline but was added after timeline position
	}
	// the timeline is scrolled and the item is older than the timeline position
	return timelinePosition.Add(1 * time.Minute) // nudge item into timeline
}

func (*Item) DatabaseObject() string {
	return "item"
}
func (*Item) DatabaseTable() string {
	return "items"
}
func (*Item) DatabaseView() string {
	return "items_view"
}
func (*Item) DatabaseInsertColumns() (columns []string) {
	return []string{
		"id",
		"url",
		"external_hrefs",
		"title",
		"summary",
		"content",
		"published_at",
		"modified_at",
		"added_at",
		"updated_at",
		"guid",
		"feed_at",
		"feed_id",
		"website_id",
	}
}
func (item *Item) DatabaseInsertValues() (args []any) {
	args = append(args, sql.Named("id", item.Identifier()))
	if item.URL != nil {
		args = append(args, sql.Named("url", item.URL.String()))
	} else {
		args = append(args, sql.Named("url", nil))
	}
	if len(item.ExternalHrefs) > 0 {
		args = append(args, sql.Named("external_hrefs", fmt.Sprintf("[%q]", strings.Join(item.ExternalHrefs, ", "))))
	} else {
		args = append(args, sql.Named("external_hrefs", "[]"))
	}
	args = append(args, sql.Named("title", item.Title))
	args = append(args, sql.Named("summary", strings.TrimSpace(item.Summary)))
	args = append(args, sql.Named("content", strings.TrimSpace(item.Content)))
	args = append(args, sql.Named("published_at", item.PublishedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("modified_at", item.ModifiedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("added_at", item.AddedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("updated_at", item.UpdatedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("guid", item.GUID))
	args = append(args, sql.Named("feed_at", item.FeedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("feed_id", item.Feed.Identifier()))
	args = append(args, sql.Named("website_id", item.Feed.Website.Identifier()))
	return args
}
func (item *Item) DatabaseUpdatesSet(_ []string) []string {
	return []string{
		"url = excluded.url",
		"external_hrefs = excluded.external_hrefs",
		"title = excluded.title",
		"summary = excluded.summary",
		"content = excluded.content",
		"modified_at = excluded.modified_at",
		"updated_at = excluded.updated_at",
		"website_id = excluded.website_id",
	}
}

func (item *Item) Hash() string {
	return fmt.Sprintf(
		"%s|%q|%s|%s|%s|%s|%s|%s",
		item.URL.String(),
		item.ExternalHrefs,
		item.Title,
		strings.TrimSpace(item.Summary),
		strings.TrimSpace(item.Content),
		item.PublishedAt.UTC().Format(time.RFC3339),
		item.ModifiedAt.UTC().Format(time.RFC3339),
		item.Feed.Identifier(),
	)
}
