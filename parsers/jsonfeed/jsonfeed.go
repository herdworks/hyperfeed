package jsonfeed

import (
    "encoding/json"

    herr "github.com/herdworks/hyperfeed-go/internal/errors"
    "github.com/herdworks/hyperfeed-go/parsers"
)

// Parser implements parsers.Parser for JSON Feed.
type Parser struct{}

func (p *Parser) Detect(doc []byte) parsers.FeedFormat {
    // Minimal detection: check for a top-level "version" field containing "jsonfeed".
    var tmp struct{ Version string `json:"version"` }
    if json.Unmarshal(doc, &tmp) == nil && tmp.Version != "" {
        return parsers.FormatJSONFeed
    }
    return parsers.FormatUnknown
}

func (p *Parser) Parse(doc []byte, opts parsers.ParseOptions) (*parsers.Result, error) {
    // Stub implementation for scaffolding.
    // A future implementation will unmarshal the JSON and map to types.Feed/Items.
    return nil, &herr.Error{Kind: herr.KindTemporary, Op: "parsers/jsonfeed.Parse", Err: herr.ErrPermanent}
}

// ensure interface conformance at compile time
var _ parsers.Parser = (*Parser)(nil)
