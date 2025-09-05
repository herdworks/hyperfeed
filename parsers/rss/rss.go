package rss

import (
    "bytes"

    herr "github.com/herdworks/hyperfeed-go/internal/errors"
    "github.com/herdworks/hyperfeed-go/parsers"
)

// Parser implements parsers.Parser for RSS.
type Parser struct{}

func (p *Parser) Detect(doc []byte) parsers.FeedFormat {
    b := bytes.TrimSpace(doc)
    // Naive detection: look for an <rss> root element.
    if bytes.Contains(b, []byte("<rss")) {
        return parsers.FormatRSS
    }
    return parsers.FormatUnknown
}

func (p *Parser) Parse(doc []byte, opts parsers.ParseOptions) (*parsers.Result, error) {
    // Stub: will implement mapping from RSS to types.
    return nil, &herr.Error{Kind: herr.KindTemporary, Op: "parsers/rss.Parse"}
}

// ensure interface conformance at compile time
var _ parsers.Parser = (*Parser)(nil)

