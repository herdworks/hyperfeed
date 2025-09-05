package atom

import (
    "bytes"

    herr "github.com/herdworks/hyperfeed-go/internal/errors"
    "github.com/herdworks/hyperfeed-go/parsers"
)

// Parser implements parsers.Parser for Atom.
type Parser struct{}

func (p *Parser) Detect(doc []byte) parsers.FeedFormat {
    b := bytes.TrimSpace(doc)
    // Naive detection: look for a <feed> root element and the Atom namespace.
    if bytes.Contains(b, []byte("<feed")) && bytes.Contains(b, []byte("http://www.w3.org/2005/Atom")) {
        return parsers.FormatAtom
    }
    return parsers.FormatUnknown
}

func (p *Parser) Parse(doc []byte, opts parsers.ParseOptions) (*parsers.Result, error) {
    // Stub: will implement mapping from Atom to types.
    return nil, &herr.Error{Kind: herr.KindTemporary, Op: "parsers/atom.Parse"}
}

// ensure interface conformance at compile time
var _ parsers.Parser = (*Parser)(nil)

