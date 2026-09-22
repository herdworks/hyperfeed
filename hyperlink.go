package hyperfeed

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"

	"github.com/herdworks/hq"
	"github.com/herdworks/hyperfeed/href"
	normalized "github.com/herdworks/hyperfeed/internal/normalization"
	"golang.org/x/net/html"
)

// Link represents a content or syndication link. Existing extraction and
// persistence continue to use its destination and enrichment fields.
type Link struct {
	*DatabaseRecord

	WebsiteID   string     `json:"website_id,omitempty"`
	ID          string     `json:"id,omitempty"`
	Href        *href.Href `json:"href,omitempty"`
	CoverHref   *href.Href `json:"cover_href,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Weight      float32    `json:"same_origin,omitempty"`

	// Syndication attributes are carried in memory and JSON only.
	Rel      string `json:"rel,omitempty"`
	Mimetype string `json:"mimetype,omitempty"`
	HrefLang string `json:"href_lang,omitempty"`
	Length   *int64 `json:"length,omitempty"` // bytes; nil means unspecified
	Origin   string `json:"origin,omitempty"` // content or metadata; empty means unspecified
}

// ParseLinks() extracts links from an item's HTML content
func ParseLinks(item *Item, htmlBytes []byte, w io.Writer) (links []*Link, err error) {
	var document hq.Document = hq.Document{}
	var ctx context.Context = context.Background()
	var perr error // failable parser errors
	if item.URL != nil {
		document.Href = item.URL.String()
		if h, e := href.Parse(item.URL.String()); e == nil {
			item.Link = &Link{Href: h, Weight: 0.25}
			if item.Feed != nil {
				item.Link.WebsiteID = item.Feed.Website.Identifier()
			}
			links = append(links, item.Link) // include a "self link"
		}
	} else {
		document.Href = "about:blank" // TODO: setting document.Href for hypergraph's Document.Ready() 🤦 until we finish the hypergraph refactor
	}
	for _, attachment := range item.Attachments {
		if attachment.Mimetype != "text/x-url" {
			continue
		}
		link := &Link{Href: attachment.Href}
		websiteURL := url.URL{Scheme: attachment.Href.URL().Scheme, Host: attachment.Href.URL().Host, Path: "/"}
		website := &Website{Entity: hq.Entity{HTMLURL: websiteURL.String()}}
		link.WebsiteID = website.Identifier()
		link.Weight = 1.00
		links = append(links, link)
	}
	if document.HTML, perr = html.Parse(bytes.NewReader(htmlBytes)); perr != nil {
		fmt.Fprintf(w, "skipping link extraction, unparseable HTML: %v\n", perr)
		return links, nil
	}
	document.Root = document.HTML
	if perr = document.Scrape(ctx, true, nil, 5, w); perr != nil {
		fmt.Fprintf(w, "skipping link extraction, scrape error: %v\n", perr)
		return links, nil
	}
	for _, i := range document.Metadata.HyperLinks.Hrefs {
		// NOTE: use a local error here to avoid returning the named `err` after a continue
		h, e := href.Parse(i)
		if e != nil {
			// log errors and continue
			fmt.Fprintf(w, "skipping unparseable link %q: %v\n", i, e)
			continue
		}
		websiteURL := url.URL{Scheme: h.URL().Scheme, Host: h.URL().Host, Path: "/"}
		website := &Website{Entity: hq.Entity{HTMLURL: websiteURL.String()}}
		links = append(links, &Link{Href: h, WebsiteID: website.Identifier(), Weight: 0.75})
	}
	for _, i := range document.Metadata.HyperLinks.SameOrigin {
		h, e := href.Parse(i)
		if e != nil {
			// log errors and continue
			fmt.Fprintf(w, "skipping unparseable link %q: %v\n", i, e)
			continue
		}
		websiteURL := url.URL{Scheme: h.URL().Scheme, Host: h.URL().Host, Path: "/"}
		website := &Website{Entity: hq.Entity{HTMLURL: websiteURL.String()}}
		links = append(links, &Link{Href: h, WebsiteID: website.Identifier(), Weight: 0.50})
	}
	return links, nil
}

func (link *Link) Identifier() string {
	if link == nil {
		return normalized.Identifier("") // the unknown identifier
	}
	if link.ID == "" {
		id := normalized.URL(link.Href.URL())
		link.ID = normalized.Identifier(id.String())
	}
	return link.ID
}

func (*Link) DatabaseObject() string {
	return "link"
}
func (*Link) DatabaseTable() string {
	return "links"
}
func (*Link) DatabaseView() string {
	return "links_view"
}
func (link *Link) DatabaseInsertColumns() (columns []string) {
	return []string{
		"website_id",
		"id",
		"href",
		"cover_href",
		"title",
		"description",
	}
}
func (link *Link) DatabaseInsertValues() (args []any) {
	args = append(args, sql.Named("website_id", link.WebsiteID))
	args = append(args, sql.Named("id", link.Identifier()))
	args = append(args, sql.Named("href", link.Href.String()))
	if link.CoverHref != nil {
		args = append(args, sql.Named("cover_href", link.CoverHref.String()))
	} else {
		args = append(args, sql.Named("cover_href", nil))
	}
	args = append(args, sql.Named("title", link.Title))
	args = append(args, sql.Named("description", link.Description))
	return args
}

func (link *Link) DatabaseUpdatesSet(_ []string) (columns []string) {
	return []string{
		"cover_href = excluded.cover_href",
		"title = excluded.title",
		"description = excluded.description",
	}
}

func (link *Link) Hash() string {
	return fmt.Sprintf(
		"%s|%s",
		link.Href.String(),
		link.WebsiteID,
	)
}
