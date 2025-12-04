package markdown

import (
	"fmt"
	"os"
	"testing"
)

func TestConvertOrderedList(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "Example post summary",
			source: "testdata/1.html",
		},
		{
			name:   "Example RSS content:encoded",
			source: "testdata/2.html",
		},
		{
			name:   "Example RSS content:encoded with linked image",
			source: "testdata/3.html",
		},
		{
			name:   "Example Atom content with complex blockquote",
			source: "testdata/4.html",
		},
		{
			name:   "Example post with sup element(s)",
			source: "testdata/5.html",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var htmlBytes []byte
			var markdownString string
			var err error
			if htmlBytes, err = os.ReadFile(tc.source); err != nil {
				t.Fatalf("Failed to read test data file: %v", err)
			}
			if markdownString, err = Convert(string(htmlBytes)); err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}
			fmt.Println(markdownString)
		})
	}
}
