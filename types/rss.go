package hyperfeed

import (
	"bytes"
	"net/url"
	"time"
)

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
	// Stub: implement mapping from RSS to types.
	return nil, nil
}
