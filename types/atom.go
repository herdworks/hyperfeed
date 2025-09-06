package hyperfeed

import (
	"bytes"
	"net/url"
	"time"
)

// AtomOptions configures Atom parsing behavior.
type AtomOptions struct {
	BaseURL         *url.URL
	Now             func() time.Time
	AllowExtensions bool
}

// AtomResult contains parsed entities and any non-fatal warnings.
type AtomResult struct {
	Feed     *Feed
	Items    Items
	Warnings []error
}

// AtomDetect returns true if the document appears to be Atom.
func AtomDetect(doc []byte) bool {
	b := bytes.TrimSpace(doc)
	return bytes.Contains(b, []byte("<feed")) && bytes.Contains(b, []byte("http://www.w3.org/2005/Atom"))
}

// AtomParse converts an Atom document into a Feed and Items.
func AtomParse(doc []byte, opts AtomOptions) (*AtomResult, error) {
	// Stub: implement mapping from Atom to types.
	return nil, nil
}
