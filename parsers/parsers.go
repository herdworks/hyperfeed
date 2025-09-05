package parsers

import (
	"net/url"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// Parser parses a raw document into HyperFeed types.
type Parser interface {
	// Detect attempts to identify the format from document bytes.
	Detect(doc []byte) FeedFormat
	// Parse converts the document into a Feed and Items.
	Parse(doc []byte, opts ParserOptions) (*ParserResult, error)
}

// FeedFormat identifies supported feed formats.
type FeedFormat string

const (
	FormatUnknown FeedFormat = "unknown"
	FormatRSS     FeedFormat = "rss"
	FormatAtom    FeedFormat = "atom"
)

// ParserOptions configures parser behavior.
type ParserOptions struct {
	// BaseURL is used to resolve relative links.
	BaseURL *url.URL
	// Now returns the current time; useful for tests.
	Now func() time.Time
	// AllowExtensions enables best-effort parsing of common non-standard fields.
	AllowExtensions bool
}

// ParserResult contains parsed entities and any non-fatal warnings.
type ParserResult struct {
	Feed     *hyperfeed.Feed
	Items    hyperfeed.Items
	Warnings []error
}
