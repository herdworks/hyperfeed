package hyperfeed

import (
	"encoding/json"
	"net/url"
	"time"
)

// JSONFeedOptions configures JSON Feed parsing behavior.
type JSONFeedOptions struct {
	BaseURL         *url.URL
	Now             func() time.Time
	AllowExtensions bool
}

// JSONFeedResult contains parsed entities and any non-fatal warnings.
type JSONFeedResult struct {
	Feed     *Feed
	Items    Items
	Warnings []error
}

// JSONFeed represents a JSON Feed document as defined by https://jsonfeed.org/version/1.1
type JSONFeed struct {
	Version     string       `json:"version"`
	Title       string       `json:"title,omitempty"`
	HomePageURL string       `json:"home_page_url,omitempty"`
	FeedURL     string       `json:"feed_url,omitempty"`
	Description string       `json:"description,omitempty"`
	UserComment string       `json:"user_comment,omitempty"`
	NextURL     string       `json:"next_url,omitempty"`
	Icon        string       `json:"icon,omitempty"`
	Favicon     string       `json:"favicon,omitempty"`
	Authors     []JSONAuthor `json:"authors,omitempty"`
	Language    string       `json:"language,omitempty"`
	Expired     bool         `json:"expired,omitempty"`
	Hubs        []JSONHub    `json:"hubs,omitempty"`
	Items       []JSONItem   `json:"items,omitempty"`
}

// JSONAuthor represents an author in a JSON Feed document.
type JSONAuthor struct {
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// JSONHub represents a hub in a JSON Feed document.
type JSONHub struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// JSONAttachment represents an attachment on a JSON Feed item.
type JSONAttachment struct {
	URL              string `json:"url,omitempty"`
	MimeType         string `json:"mime_type,omitempty"`
	Title            string `json:"title,omitempty"`
	SizeInBytes      int64  `json:"size_in_bytes,omitempty"`
	DurationInSecond int64  `json:"duration_in_seconds,omitempty"`
}

// JSONItem represents an individual entry in a JSON Feed document.
type JSONItem struct {
	ID            string           `json:"id"`
	URL           string           `json:"url,omitempty"`
	ExternalURL   string           `json:"external_url,omitempty"`
	Title         string           `json:"title,omitempty"`
	ContentHTML   string           `json:"content_html,omitempty"`
	ContentText   string           `json:"content_text,omitempty"`
	Summary       string           `json:"summary,omitempty"`
	Image         string           `json:"image,omitempty"`
	BannerImage   string           `json:"banner_image,omitempty"`
	DatePublished string           `json:"date_published,omitempty"`
	DateModified  string           `json:"date_modified,omitempty"`
	Authors       []JSONAuthor     `json:"authors,omitempty"`
	Tags          []string         `json:"tags,omitempty"`
	Language      string           `json:"language,omitempty"`
	Attachments   []JSONAttachment `json:"attachments,omitempty"`
}

// JSONFeedDetect returns true if the document appears to be JSON Feed.
func JSONFeedDetect(doc []byte) bool {
	var tmp struct {
		Version string `json:"version"`
	}
	return json.Unmarshal(doc, &tmp) == nil && tmp.Version != ""
}

// JSONFeedParse converts a JSON Feed document into a Feed and Items.
func JSONFeedParse(doc []byte, opts JSONFeedOptions) (*JSONFeedResult, error) {
	var jf JSONFeed
	if err := json.Unmarshal(doc, &jf); err != nil {
		return nil, err
	}
	feed, items, warnings := jf.ToFeed(opts)
	return &JSONFeedResult{Feed: feed, Items: items, Warnings: warnings}, nil
}

// ToFeed converts a JSONFeed into core Feed and Items types.
func (jf *JSONFeed) ToFeed(opts JSONFeedOptions) (*Feed, Items, []error) {
	if jf == nil {
		return nil, nil, nil
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	var warnings []error
	feed := &Feed{
		Title:       jf.Title,
		Description: jf.Description,
		Language:    jf.Language,
		Metadata:    Metadata{},
	}
	if jf.FeedURL != "" {
		if u, err := resolveURL(opts.BaseURL, jf.FeedURL); err != nil {
			warnings = append(warnings, err)
		} else {
			feed.URL = u
		}
	}
	items := make(Items, 0, len(jf.Items))
	for _, ji := range jf.Items {
		item := &Item{
			ID:         ji.ID,
			Title:      ji.Title,
			Summary:    ji.Summary,
			Categories: ji.Tags,
			Metadata:   Metadata{},
		}
		if ji.URL != "" {
			if u, err := resolveURL(opts.BaseURL, ji.URL); err != nil {
				warnings = append(warnings, err)
			} else {
				item.URL = u
			}
		}
		if ji.ContentHTML != "" {
			item.Content = ji.ContentHTML
		} else if ji.ContentText != "" {
			item.Content = ji.ContentText
		}
		for _, ja := range ji.Authors {
			author := Author{Name: ja.Name}
			if ja.URL != "" {
				if u, err := resolveURL(opts.BaseURL, ja.URL); err != nil {
					warnings = append(warnings, err)
				} else {
					author.URI = u
				}
			}
			item.Authors = append(item.Authors, author)
		}
		if ji.DatePublished != "" {
			if t, err := parseTime(ji.DatePublished); err != nil {
				warnings = append(warnings, err)
				item.Published = now()
			} else {
				item.Published = t
			}
		}
		if ji.DateModified != "" {
			if t, err := parseTime(ji.DateModified); err != nil {
				warnings = append(warnings, err)
			} else {
				item.Updated = t
			}
		}
		if ji.Image != "" {
			if u, err := resolveURL(opts.BaseURL, ji.Image); err != nil {
				warnings = append(warnings, err)
			} else {
				item.Image = &Image{URL: u}
			}
		}
		for _, att := range ji.Attachments {
			var enc Enclosure
			if att.URL != "" {
				if u, err := resolveURL(opts.BaseURL, att.URL); err != nil {
					warnings = append(warnings, err)
				} else {
					enc.URL = u
				}
			}
			enc.Type = att.MimeType
			enc.Length = att.SizeInBytes
			item.Enclosures = append(item.Enclosures, enc)
		}
		items = append(items, item)
	}
	feed.Items = items
	return feed, items, warnings
}

func resolveURL(base *url.URL, ref string) (*url.URL, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return u, nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}
