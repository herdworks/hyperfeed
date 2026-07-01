package hyperfeed

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
	opml "github.com/herdworks/hyperfeed-go/types/opml"
)

type SubscriptionList struct {
	*DatabaseRecord

	ID string `json:"id,omitempty"` // stable HyperFeed SubscriptionList identifier (e.g. hash of URI+owner)

	// URI is the location of the OPML document representing the subscription
	// list (a file://, http://, or https:// URL; see reader.Normalize).
	URI *href.Href `json:"uri,omitempty"`

	// OPML holds the parsed OPML document.
	OPML *opml.OPML `json:"-"`

	// Standard metadata properties
	Title     string    `json:"title,omitempty"`
	Owner     Owner     `json:"owner,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`

	// Feeds contains the list of feeds in the subscription list.
	Feeds []*Feed `json:"feeds,omitempty"`

	// Status holds runtime status information about the subscription list.
	Status *SubscriptionStatus `json:"status,omitempty"`
}

type Owner struct {
	// Standard properties
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`

	// Non-standard/extension properties
	Metadata Metadata `json:"metadata,omitempty"`
}

type SubscriptionStatus struct {
	Duration       time.Duration `json:"duration,omitempty"`
	UpdatedAt      time.Time     `json:"updated_at,omitempty"`
	FeedsSkipped   Counter       `json:"feeds_skipped,omitempty"`
	FeedsFailed    Counter       `json:"feeds_failed,omitempty"`
	FeedsFetched   Counter       `json:"feeds_fetched,omitempty"`
	ItemsFetched   Counter       `json:"items_fetched,omitempty"`
	ItemsDuplicate Counter       `json:"items_duplicate,omitempty"`
	ItemsAdded     Counter       `json:"items_added,omitempty"`
	ItemsUpdated   Counter       `json:"items_updated,omitempty"`
}

// =========================== SubscriptionList Methods =======================

// Parse parses pre-read OPML bytes into the SubscriptionList (no I/O).
// Callers are responsible for reading the bytes from sl.URI (local file or
// remote URL), e.g. via the reader package.
func (sl *SubscriptionList) Parse(data []byte) (err error) {
	sl.OPML = &opml.OPML{}
	if err = sl.OPML.Unmarshal(data); err != nil {
		return err
	}
	sl.Title = sl.OPML.Head.Title
	sl.Owner.ID = sl.OPML.Head.OwnerId
	sl.Owner.Name = sl.OPML.Head.OwnerName
	sl.Owner.Email = sl.OPML.Head.OwnerEmail
	if sl.CreatedAt, err = sl.OPML.DateCreated(); err != nil {
		sl.CreatedAt = time.Now()
	}
	if sl.UpdatedAt, err = sl.OPML.DateModified(); err != nil {
		sl.UpdatedAt = sl.CreatedAt
	}
	return nil
}

// InitFeeds interprets the parsed OPML outlines as the subscription-list
// pattern, returning one Feed per subscription outline. This is where
// subscription-list rules live, as distinct from the OPML spec (which is a
// general-purpose outlining format).
//
// Subscription-list feeds are always http/https. Outlines without xmlUrl are
// treated as folders/categories and descended into. Outlines whose feed URL is
// not http/https (e.g. file://) are skipped, so OPML import can never introduce
// a local-file feed regardless of where the OPML itself was read from. Local
// feeds enter the system only via direct database access.
func (sl *SubscriptionList) InitFeeds(w io.Writer) (feeds []*Feed, err error) {
	if w == nil {
		w = os.Stdout
	}
	var walk func(outline *opml.Outline) error
	walk = func(outline *opml.Outline) error {
		if outline == nil {
			return nil
		}
		xmlURL := strings.TrimSpace(outline.XMLURL)
		if xmlURL != "" {
			var feed *Feed = &Feed{
				Title: outline.Text,
			}
			if feed.URL, err = href.Parse(xmlURL); err != nil {
				return err
			}
			if s := feed.URL.URL().Scheme; s != "http" && s != "https" {
				fmt.Fprintf(w, "subscription skipped: non-web feed URL %q (OPML feeds must be http/https)\n", xmlURL)
			} else {
				if htmlURL := strings.TrimSpace(outline.HTMLURL); htmlURL != "" {
					feed.HTMLURL, _ = href.Parse(htmlURL)
					feed.HTMLURL = href.ResolveHref(feed.URL, feed.HTMLURL, w)
				}
				// Imported feeds are followed. `following` is a written column now, so a
				// missing (zero-value false) would otherwise persist as unfollowed.
				feed.Following = true
				feed.Website.Following = true
				feed.Status.LastStatusCodes = []int{}
				feeds = append(feeds, feed)
			}
		}
		for i := range outline.Outlines {
			if err := walk(&outline.Outlines[i]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, outline := range sl.OPML.Body.Outlines {
		if err = walk(outline); err != nil {
			return feeds, err
		}
	}
	return feeds, nil
}

// MarshalOPML renders the SubscriptionList as an OPML subscription-list
// document — one subscription outline per Feed — returning indented OPML XML
// bytes. The inverse of Parse + InitFeeds. When redacted is true, feed URLs
// are emitted with credentials removed.
func (sl *SubscriptionList) MarshalOPML(redacted bool) (out []byte, err error) {
	var doc opml.OPML = opml.OPML{
		Version: "2.0",
		Head: opml.Head{
			Title:      sl.Title,
			OwnerId:    sl.Owner.ID,
			OwnerName:  sl.Owner.Name,
			OwnerEmail: sl.Owner.Email,
		},
	}
	if !sl.CreatedAt.IsZero() {
		doc.Head.DateCreated = sl.CreatedAt.Format(time.RFC1123Z)
	}
	if !sl.UpdatedAt.IsZero() {
		doc.Head.DateModified = sl.UpdatedAt.Format(time.RFC1123Z)
	}
	for _, feed := range sl.Feeds {
		var outline opml.Outline = opml.Outline{
			Text:        feed.Title,
			Title:       feed.Title,
			Description: feed.Description,
			Type:        feed.Kind,
			XMLURL:      feed.URL.String(),
			HTMLURL:     feed.HTMLURL.String(),
		}
		if redacted {
			// RedactedURL returns a nil *url.URL for a nil Href; fall back to
			// the nil-safe Href.String() value already set above.
			if u := feed.URL.RedactedURL(); u != nil {
				outline.XMLURL = u.String()
			}
		}
		doc.Body.Outlines = append(doc.Body.Outlines, &outline)
	}
	return doc.MarshalIndent("", "\t")
}

func (subscription *SubscriptionList) Identifier() string {
	if subscription == nil {
		return ""
	}
	if subscription.ID == "" && subscription.URI != nil {
		subscription.ID = normalized.Identifier(subscription.URI.String())
	}
	return subscription.ID
}
func (*SubscriptionList) DatabaseObject() string {
	return "subscriptionlist"
}
func (*SubscriptionList) DatabaseTable() string {
	return "subscriptionlists"
}
func (*SubscriptionList) DatabaseView() string {
	return "subscriptionlists_view"
}
func (*SubscriptionList) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"uri",
		"title",
		"owner_name",
		"owner_email",
		"created_at",
		"updated_at",
		"status",
	}
}
func (subscription *SubscriptionList) DatabaseInsertValues() (args []any) {
	statusBytes, _ := json.Marshal(subscription.Status)
	args = append(args, sql.Named("id", subscription.Identifier()))
	args = append(args, sql.Named("uri", subscription.URI.String()))
	args = append(args, sql.Named("title", subscription.Title))
	args = append(args, sql.Named("owner_name", subscription.Owner.Name))
	args = append(args, sql.Named("owner_email", subscription.Owner.Email))
	args = append(args, sql.Named("created_at", subscription.CreatedAt.UTC().Format(time.RFC3339)))
	if subscription.UpdatedAt.IsZero() {
		args = append(args, sql.Named("created_at", subscription.CreatedAt.UTC().Format(time.RFC3339)))
	} else {
		args = append(args, sql.Named("updated_at", subscription.UpdatedAt.UTC().Format(time.RFC3339)))
	}
	args = append(args, sql.Named("status", string(statusBytes))) // status
	return args
}
func (subscription *SubscriptionList) DatabaseUpdatesSet(_ []string) []string {
	return []string{
		"uri = excluded.uri",
		"title = excluded.title",
		"owner_name = excluded.owner_name",
		"owner_email = excluded.owner_email",
		// "created_at = excluded.created_at", // never update created_at
		"updated_at = excluded.updated_at",
		"status = excluded.status",
	}
}

func (subscription *SubscriptionList) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s",
		subscription.Identifier(),
		subscription.URI.String(),
		subscription.Title,
		subscription.CreatedAt.UTC().Format(time.RFC3339),
	)
}
