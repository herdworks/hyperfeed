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
		{
			name:   "Example post with numerical HTML character entities",
			source: "testdata/6.html",
		},
		{
			name:   "Pluralistic MESS",
			source: "testdata/7.html",
		},
		{
			name:   "Daring Fireball multimedia post",
			source: "testdata/8.html",
		},
		{
			name:   "HTMHell post",
			source: "testdata/9.html",
		},
		{
			name:   "Vergecast episode description (plaintext)",
			source: "testdata/10.html",
		},
		{
			name:   "GitHub blog post with video poster",
			source: "testdata/11.html",
		},
		{
			name:   "HTMHell",
			source: "testdata/htmhell.html",
		},
		{
			name:   "GoMakeThings",
			source: "testdata/gomakethings.html",
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
			fmt.Println()
			fmt.Println()
		})
	}
}
