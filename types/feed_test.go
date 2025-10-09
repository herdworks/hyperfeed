package hyperfeed

import (
	"os"
	"testing"
)

func TestDetectKind(t *testing.T) {
	var testdatadir string = "testdata/2025-09-09/feeds/" // list files in testdata/2025-09-09/feeds/ and try to parse each one
	var files []os.DirEntry
	var err error
	if files, err = os.ReadDir(testdatadir); err != nil {
		t.Fatalf("Failed to read test data directory: %v", err)
	}
	for _, f := range files {
		var feed Feed = Feed{}
		if f.IsDir() {
			continue
		}
		if feed.Bytes, err = os.ReadFile(testdatadir + f.Name()); err != nil {
			t.Fatalf("Failed to read feed file %q: %v", f.Name(), err)
		}
		feedKind := feed.DetectKind()
		if feedKind == "" {
			t.Logf("Could not detect kind for feed file %q", f.Name())
			continue
		}
		if err = feed.Parse(); err != nil {
			t.Logf("Parse error for feed file %q: %v", f.Name(), err)
			continue
		}
		if feed.RawFeed == nil {
			t.Logf("No RawFeed for %q feed file %q", feedKind, f.Name())
		}
		t.Logf("Kind %q for feed file %q (%d items)", feedKind, f.Name(), feed.RawFeed.ItemCount())
	}
}

func TestFeedRowCompliance(t *testing.T) {
	var feed Feed = Feed{}
	if err := RowComplianceTest(&feed); err != nil {
		t.Fatalf("Feed Row compliance failed: %v", err)
	}
}
