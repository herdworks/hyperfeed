package hyperfeed

import (
	"os"
	"testing"

	"github.com/herdworks/hyperfeed-go/types/atom"
	"github.com/herdworks/hyperfeed-go/types/jsonfeed"
	"github.com/herdworks/hyperfeed-go/types/rss"
)

func TestDetectKind(t *testing.T) {
	var testdatadir string = "testdata/2025-09-09/feeds/" // list files in testdata/2025-09-09/feeds/ and try to parse each one
	var files []os.DirEntry
	var err error
	if files, err = os.ReadDir(testdatadir); err != nil {
		t.Fatalf("Failed to read test data directory: %v", err)
	}
	for _, f := range files {
		var feed *Feed = &Feed{}
		var feedKind string
		if f.IsDir() {
			continue
		}
		if feed.Bytes, err = os.ReadFile(testdatadir + f.Name()); err != nil {
			t.Fatalf("Failed to read feed file %q: %v", f.Name(), err)
		}
		switch {
		case rss.Detected(feed.Bytes):
			feedKind = "rss"
		case atom.Detected(feed.Bytes):
			feedKind = "atom"
		case jsonfeed.Detected(feed.Bytes):
			feedKind = "jsonfeed"
		default:
			t.Logf("Could not detect kind for feed file %q", f.Name())
		}
		t.Logf("Detected feed kind %q for feed file %q", feedKind, f.Name())
	}
}
