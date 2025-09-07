package hyperfeed

import (
	"bytes"
	"encoding/xml"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/normalization"
)

// RSS represents an RSS 2.0 feed.
type RSS struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	PubDate       string    `xml:"pubDate"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	Author      string        `xml:"author"`
	GUID        string        `xml:"guid"`
	PubDate     string        `xml:"pubDate"`
	Categories  []string      `xml:"category"`
	Enclosure   *rssEnclosure `xml:"enclosure"`
}

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// RSSOptions configures RSS parsing behavior.
type RSSOptions struct {
	BaseURL         *url.URL
	Now             func() time.Time
	AllowExtensions bool
}

// RSSResult contains parsed entities and any non-fatal warnings.
type RSSResult struct {
	Feed     *Feed
	Items    Items
	Warnings []error
}

// RSSDetect returns true if the document appears to be RSS.
func RSSDetect(doc []byte) bool {
	b := bytes.TrimSpace(doc)
	return bytes.Contains(b, []byte("<rss"))
}

// RSSParse converts an RSS document into a Feed and Items.
func RSSParse(doc []byte, opts RSSOptions) (*RSSResult, error) {
	var r RSS
	if err := xml.Unmarshal(doc, &r); err != nil {
		return nil, err
	}
	feed, items := r.toFeed(opts)
	return &RSSResult{Feed: feed, Items: items}, nil
}

func (r *RSS) toFeed(opts RSSOptions) (*Feed, Items) {
	feed := &Feed{
		URL:         normalization.URL(opts.BaseURL),
		Title:       r.Channel.Title,
		Description: r.Channel.Description,
		Language:    r.Channel.Language,
	}
	if t, err := parseRSSDate(r.Channel.LastBuildDate); err == nil {
		feed.Updated = t
	} else if t, err := parseRSSDate(r.Channel.PubDate); err == nil {
		feed.Updated = t
	} else if opts.Now != nil {
		feed.Updated = opts.Now()
	}

	items := make(Items, 0, len(r.Channel.Items))
	for _, it := range r.Channel.Items {
		item := &Item{
			GUID:       it.GUID,
			Title:      it.Title,
			Summary:    it.Description,
			Categories: it.Categories,
		}
		if it.Link != "" {
			item.URL = resolveURL(it.Link, opts.BaseURL)
		}
		if it.Author != "" {
			item.Authors = []Author{{Name: it.Author}}
		}
		if t, err := parseRSSDate(it.PubDate); err == nil {
			item.Published = t
		}
		if it.Enclosure != nil {
			enc := Enclosure{Type: it.Enclosure.Type}
			enc.URL = resolveURL(it.Enclosure.URL, opts.BaseURL)
			if l, err := strconv.ParseInt(it.Enclosure.Length, 10, 64); err == nil {
				enc.Length = l
			}
			item.Enclosures = []Enclosure{enc}
		}
		items = append(items, item)
	}
	feed.Items = items
	return feed, items
}

func resolveURL(ref string, base *url.URL) *url.URL {
	if ref == "" {
		return nil
	}
	u, err := url.Parse(ref)
	if err != nil {
		return nil
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return normalization.URL(u)
}

func parseRSSDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unsupported time format")
}
