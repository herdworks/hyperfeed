package hyperfeed

import (
    "net/url"
    "time"
)

// Feed represents a single RSS/Atom feed.
type Feed struct {
    // ID is an optional stable identifier for the feed (e.g., a UUID or hash).
    ID string `json:"id,omitempty"`

    // URL is the canonical feed URL (RSS/Atom endpoint).
    URL *url.URL `json:"url,omitempty"`

    // SiteURL is the website/homepage associated with the feed.
    SiteURL *url.URL `json:"site_url,omitempty"`

    Title       string    `json:"title,omitempty"`
    Description string    `json:"description,omitempty"`
    Language    string    `json:"language,omitempty"`
    Updated     time.Time `json:"updated,omitempty"`

    Image *Image `json:"image,omitempty"`

    // Items contains the most recent items for this feed.
    Items Items `json:"items,omitempty"`

    // Metadata holds extension data for other packages (scheduler, subscriber, etc.).
    Metadata Metadata `json:"metadata,omitempty"`
}

type Feeds []*Feed

// CanonicalID returns a stable identifier for the feed, preferring ID then URL.
func (f *Feed) CanonicalID() string {
    if f == nil {
        return ""
    }
    if f.ID != "" {
        return f.ID
    }
    if f.URL != nil {
        return f.URL.String()
    }
    return ""
}
