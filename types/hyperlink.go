package hyperfeed

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
	hypergraph "github.com/herdworks/hypergraph-go/types"
	"golang.org/x/net/html"
)

// HyperLink represents an external link found in an item.
type HyperLink struct {
	*DatabaseRecord

	ID        string     `json:"id,omitempty"`
	Href      *href.Href `json:"href,omitempty"`
	WebsiteID string     `json:"website_id,omitempty"` // back-reference to parent website
}

// ParseLinks() extracts links from an item's HTML content
func ParseLinks(item *Item, htmlBytes []byte, w io.Writer) (links []*HyperLink) {
	var document hypergraph.Document = hypergraph.Document{}
	var ctx context.Context = context.Background()
	var err error
	if item.URL != nil {
		document.Href = item.URL.String()
		if h, e := href.Parse(item.URL.String()); e == nil {
			l := &HyperLink{Href: h}
			if item.Feed != nil {
				l.WebsiteID = item.Feed.Website.Identifier()
			}
			links = append(links, l) // include a "self link"
		}
	} else {
		document.Href = "about:blank" // TODO: setting document.Href for hypergraph's Document.Ready() 🤦 until we finish the hypergraph refactor
	}
	if document.HTML, err = html.Parse(bytes.NewReader(htmlBytes)); err == nil {
		document.Root = document.HTML
	} else {
		return links
	}
	if err = document.Scrape(ctx, true, nil, 5, w); err != nil {
		return links
	}
	for _, i := range document.Metadata.HyperLinks.Hrefs {
		var h *href.Href
		var website *Website
		if h, err = href.Parse(i); err != nil {
			continue
		}
		websiteURL := url.URL{Scheme: h.URL().Scheme, Host: h.URL().Host, Path: "/"}
		website = &Website{Entity: hypergraph.Entity{HTMLURL: websiteURL.String()}}
		links = append(links, &HyperLink{Href: h, WebsiteID: website.Identifier()})
	}
	return links
}

func (link *HyperLink) Identifier() string {
	if link == nil {
		return normalized.Identifier("") // the unknown identifier
	}
	if link.ID == "" {
		id := normalized.URL(link.Href.URL())
		link.ID = normalized.Identifier(id.String())
	}
	return link.ID
}

func (*HyperLink) DatabaseObject() string {
	return "link"
}
func (*HyperLink) DatabaseTable() string {
	return "links"
}
func (*HyperLink) DatabaseView() string {
	return "links_view"
}
func (link *HyperLink) DatabaseInsertColumns() (columns []string) {
	return []string{
		"id",
		"href",
		"website_id",
	}
}
func (link *HyperLink) DatabaseInsertValues() (args []any) {
	args = append(args, sql.Named("id", link.Identifier()))
	args = append(args, sql.Named("href", link.Href.String()))
	args = append(args, sql.Named("website_id", link.WebsiteID))
	return args
}

func (link *HyperLink) Hash() string {
	return fmt.Sprintf(
		"%s|%s",
		link.Href.String(),
		link.WebsiteID,
	)
}
