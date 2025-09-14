package hyperfeed

import (
	"net/url"
	"path/filepath"
	"testing"
)

func TestOPMLLoad(t *testing.T) {
	opml := &OPML{URI: &url.URL{Path: filepath.Join("testdata", "2025-09-09", "nnw_default_subscriptions.opml")}}
	if err := opml.Load(); err != nil {
		t.Fatalf("Failed to load OPML: %v", err)
	}
	if len(opml.Body.Outlines) != 14 {
		t.Fatalf("expected 14 outlines; got %d", len(opml.Body.Outlines))
	}
}
