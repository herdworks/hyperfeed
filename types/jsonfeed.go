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

// JSONFeedDetect returns true if the document appears to be JSON Feed.
func JSONFeedDetect(doc []byte) bool {
	var tmp struct {
		Version string `json:"version"`
	}
	return json.Unmarshal(doc, &tmp) == nil && tmp.Version != ""
}

// JSONFeedParse converts a JSON Feed document into a Feed and Items.
func JSONFeedParse(doc []byte, opts JSONFeedOptions) (*JSONFeedResult, error) {
	// Stub: implement mapping from JSON Feed to types.
	return nil, nil
}
