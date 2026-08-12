package opml

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
	}{
		{
			name:     "NNW default subscriptions",
			filepath: filepath.Join("testdata", "nnw_default_subscriptions.opml"),
		},
		{
			name:     "Sample OPML",
			filepath: filepath.Join("testdata", "hyperfeed.opml"),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(tc.filepath)
			if err != nil {
				t.Fatalf("Failed to read OPML: %v", err)
			}
			opml, err := Parse(data)
			if err != nil {
				t.Fatalf("Failed to parse OPML: %v", err)
			}
			t.Logf("opml %q has %d outlines", tc.filepath, len(opml.Body.Outlines))
		})
	}
}

func TestOPML_DateCreated(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		layout  string
		dateStr string
	}{
		{
			name:    "RFC822 with zone name",
			layout:  "Mon, 02 Jan 06 15:04 MST",
			dateStr: "Mon, 02 Jan 06 15:04 MST",
		},
		{
			name:    "RFC822 without weekday, zone name",
			layout:  "02 Jan 06 15:04 MST",
			dateStr: "02 Jan 06 15:04 MST",
		},
		{
			name:    "RFC822Z with numeric offset",
			layout:  "Mon, 02 Jan 06 15:04 -0700",
			dateStr: "Mon, 02 Jan 06 15:04 -0700",
		},
		{
			name:    "RFC822Z without weekday, numeric offset",
			layout:  "02 Jan 06 15:04 -0700",
			dateStr: "02 Jan 06 15:04 -0700",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var want time.Time
			var got time.Time
			var err error
			t.Parallel()

			if want, err = time.Parse(tc.layout, tc.dateStr); err != nil {
				t.Fatalf("test setup parse failed for %q: %v", tc.dateStr, err)
			}

			op := &OPML{Head: Head{DateCreated: tc.dateStr, DateModified: tc.dateStr}}

			// DateCreated test
			if got, err = op.DateCreated(); err != nil {
				t.Fatalf("DateCreated() error = %v, want nil", err)
			}
			if !got.Equal(want) {
				t.Fatalf("DateCreated() = %v, want %v", got, want)
			}

			// DateModified test
			if got, err = op.DateModified(); err != nil {
				t.Fatalf("DateModified() error = %v, want nil", err)
			}
			if !got.Equal(want) {
				t.Fatalf("DateModified() = %v, want %v", got, want)
			}
		})
	}
}
