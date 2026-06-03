package hyperfeed

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// ============================ Package Types =================================

// Item represents an entry within a feed.
type Item struct {
	*DatabaseRecord

	ID          string     `json:"id,omitempty"`
	URL         *href.Href `json:"url,omitempty"`          // URL is the canonical link for the item (permalink).
	CommentsURL *href.Href `json:"comments_url,omitempty"` // CommentsURL is the URL for the item's comments page (if available)

	PublishedAt time.Time `json:"published_at,omitempty"` // source item created timestamp
	ModifiedAt  time.Time `json:"modified_at,omitempty"`  // source item last modified timestamp
	AddedAt     time.Time `json:"added_at,omitempty"`     // when we first saw the item
	UpdatedAt   time.Time `json:"updated_at,omitempty"`   // when we last updated the item in our database
	FeedAt      time.Time `json:"feed_at,omitempty"`      // default feed order timestamp

	Title            string       `json:"title,omitempty"`
	Bylines          []Byline     `json:"bylines,omitempty"`
	Summary          string       `json:"summary,omitempty"`
	Content          string       `json:"content,omitempty"`
	Links            []*HyperLink `json:"links,omitempty"`
	Attachments      []Attachment `json:"attachments,omitempty"` // RSS enclosures, Atom enclosure Links, JSONFeed attachments, and scraped media (images, videos, audio, documents, and other files)
	Categories       []string     `json:"categories,omitempty"`
	IsPodcastEpisode bool         `json:"is_podcast_episode,omitempty"`

	// Metadata holds non-standard/extension Item properties
	Metadata Metadata `json:"metadata,omitempty"`

	// GUID is the original feed-provided unique identifier for the item
	// GUID is only used as Identifier() material
	GUID string `json:"-"`

	// Feed is the parent feed this item belongs to
	Feed *Feed      `json:"-"` // parent feed (not serialized)
	Link *HyperLink `json:"-"` // primary link (not serialized)
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
		u := normalized.URL(item.URL.URL())
		item.ID = normalized.Identifier(fmt.Sprintf("%s/%s/%s", item.Feed.WebsiteIdentifier(), u, item.GUID))
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
		item.URL = href.ResolveHref(base, item.URL, w)
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
	for i, a := range item.Attachments {
		if a.Href != nil {
			a.Href = href.ResolveHref(base, a.Href, w)
			item.Attachments[i] = a
		}
		for i, src := range a.Sources {
			if src.Href != nil {
				src.Href = href.ResolveHref(base, src.Href, w)
				a.Sources[i] = src
			}
		}
	}
}

// TimelineAt computes the HyperFeed default "chronological" timeline timestamp for an item
//
// TimelineAt uses the following logic:
// - An item's actual timestamp is determined by the earliest of PublishedAt and ModifiedAt
// - We should always use the item's actual timestamp if it's not "zero" and not in the future
// - If an item's actual timestamp is in the future, use AddedAt
// - If an item's actual timestamp is "zero" (missing), and it's a new feed, use the Unix epoch - offset
// - If an item's actual timestamp is "zero" (missing), but the feed is not new, use AddedAt - offset
// - If no other condition applies, use the AddedAt timestamp
func (item *Item) TimelineAt(offset int) (ts time.Time) {
	newFeed := count(item.Feed.Status.LastStatusCodes, 200, 304) == 1 // if the feed has only ever returned one successful response, it's our first time seeing items from this feed
	ts = Earliest(item.PublishedAt, item.ModifiedAt)                  // actual item timestamp
	if offset > 0 {
		offset = -offset // use a negative offset to retain the original order of items with "zero" timestamps (within a given feed)
	}
	if !ts.IsZero() && ts.Before(time.Now()) {
		return ts.UTC() // use item timestamp if it's not "zero" and not in the future
	}
	if ts.After(time.Now()) {
		return item.AddedAt.UTC() // item has invalid future added timestamp, use added timestamp
	}
	days := time.Duration((offset - 1)) * (time.Hour * 24)
	if ts.IsZero() && newFeed {
		return time.Unix(0, 0).Add(days).UTC() // if the feed is new and the item is missing a timestamp, use the Unix epoch with offset to preserve deterministic order of items in a new feed
	} else if ts.IsZero() {
		return item.AddedAt.Add(days).UTC() // item is missing a timestamp, use added timestamp with offset to preserve deterministic order of items in a given feed
	}
	return item.AddedAt.UTC() // default to added timestamp
}

func (item *Item) ContainsPodcastMedia() bool {
	if item == nil {
		return false
	}
	for _, attachment := range item.Attachments {
		if strings.HasPrefix(attachment.Mimetype, "audio/") || strings.HasPrefix(attachment.Mimetype, "video/") {
			return true
		}
	}
	return false
}

func (item *Item) DeduplicateAttachments() {
	if item == nil || len(item.Attachments) < 2 {
		return
	}
	attachments := []Attachment{}
	seen := map[string]int{}
	for _, attachment := range item.Attachments {
		if attachment.Href == nil || attachment.Href.String() == "" {
			attachments = append(attachments, attachment)
			continue
		}
		key := normalized.URL(attachment.Href.URL()).String()
		if index, ok := seen[key]; ok {
			attachments[index] = mergeAttachments(attachments[index], attachment)
			continue
		}
		seen[key] = len(attachments)
		attachments = append(attachments, attachment)
	}
	item.Attachments = attachments
}

func mergeAttachments(existing, incoming Attachment) Attachment {
	existing.Enclosure = existing.Enclosure || incoming.Enclosure
	existing.Download = existing.Download || incoming.Download
	if existing.ID == "" {
		existing.ID = incoming.ID
	}
	if existing.Href == nil {
		existing.Href = incoming.Href
	}
	if existing.Poster == nil {
		existing.Poster = incoming.Poster
	}
	if incoming.Mimetype != "" && (existing.Mimetype == "" || existing.Mimetype == "application/octet-stream") {
		existing.Mimetype = incoming.Mimetype
	}
	if existing.Title == "" {
		existing.Title = incoming.Title
	}
	if existing.Description == "" {
		existing.Description = incoming.Description
	}
	if existing.Size == 0 {
		existing.Size = incoming.Size
	}
	if existing.Duration == 0 {
		existing.Duration = incoming.Duration
	}
	if existing.PlaybackPosition == 0 {
		existing.PlaybackPosition = incoming.PlaybackPosition
	}
	if existing.Width == 0 {
		existing.Width = incoming.Width
	}
	if existing.Height == 0 {
		existing.Height = incoming.Height
	}
	existing.Sources = mergeAttachmentSources(existing.Sources, incoming.Sources)
	return existing
}

func mergeAttachmentSources(existing, incoming []Source) []Source {
	seen := map[string]int{}
	for index, source := range existing {
		if source.Href == nil || source.Href.String() == "" {
			continue
		}
		seen[normalized.URL(source.Href.URL()).String()] = index
	}
	for _, source := range incoming {
		if source.Href != nil && source.Href.String() != "" {
			key := normalized.URL(source.Href.URL()).String()
			if index, ok := seen[key]; ok {
				if source.Mimetype != "" && source.Mimetype != "application/octet-stream" && (existing[index].Mimetype == "" || existing[index].Mimetype == "application/octet-stream") {
					existing[index].Mimetype = source.Mimetype
				}
				if existing[index].Width == 0 {
					existing[index].Width = source.Width
				}
				if existing[index].Height == 0 {
					existing[index].Height = source.Height
				}
				if existing[index].Container == "" {
					existing[index].Container = source.Container
				}
				continue
			}
			seen[key] = len(existing)
		}
		existing = append(existing, source)
	}
	return existing
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
		"comments_url",
		"title",
		"summary",
		"content",
		"is_podcast_episode",
		"published_at",
		"modified_at",
		"added_at",
		"updated_at",
		"guid",
		"feed_at",
		"feed_id",
		"website_id",
		"link_id",
	}
}
func (item *Item) DatabaseInsertValues() (args []any) {
	args = append(args, sql.Named("id", item.Identifier()))
	if item.URL != nil {
		args = append(args, sql.Named("url", item.URL.String()))
	} else {
		args = append(args, sql.Named("url", nil))
	}
	if item.CommentsURL != nil {
		args = append(args, sql.Named("comments_url", item.CommentsURL.String()))
	} else {
		args = append(args, sql.Named("comments_url", nil))
	}
	args = append(args, sql.Named("title", item.Title))
	args = append(args, sql.Named("summary", strings.TrimSpace(item.Summary)))
	args = append(args, sql.Named("content", strings.TrimSpace(item.Content)))
	args = append(args, sql.Named("is_podcast_episode", item.IsPodcastEpisode))
	args = append(args, sql.Named("published_at", item.PublishedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("modified_at", item.ModifiedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("added_at", item.AddedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("updated_at", item.UpdatedAt.UTC().Format(time.RFC3339)))
	if item.GUID == "" {
		args = append(args, sql.Named("guid", nil))
	} else {
		args = append(args, sql.Named("guid", item.GUID))
	}
	args = append(args, sql.Named("feed_at", item.FeedAt.UTC().Format(time.RFC3339)))
	args = append(args, sql.Named("feed_id", item.Feed.Identifier()))
	args = append(args, sql.Named("website_id", item.Feed.Website.Identifier()))
	args = append(args, sql.Named("link_id", item.Link.Identifier()))
	return args
}
func (item *Item) DatabaseUpdatesSet(_ []string) []string {
	return []string{
		"url = excluded.url",
		"comments_url = excluded.comments_url",
		"title = excluded.title",
		"summary = excluded.summary",
		"content = excluded.content",
		"is_podcast_episode = excluded.is_podcast_episode",
		"modified_at = excluded.modified_at",
		"updated_at = excluded.updated_at",
		"website_id = excluded.website_id",
		"link_id = excluded.link_id",
	}
}

func (item *Item) Hash() string {
	return fmt.Sprintf(
		"%s|%q|%s|%s|%s|%s|%s",
		item.URL.String(),
		item.Title,
		strings.TrimSpace(item.Summary),
		strings.TrimSpace(item.Content),
		item.PublishedAt.UTC().Format(time.RFC3339),
		item.ModifiedAt.UTC().Format(time.RFC3339),
		item.Feed.Identifier(),
	)
}
