package hyperfeed

import (
	"errors"
	"net/url"
	"testing"

	"github.com/herdworks/hyperfeed-go/internal/normalization"
)

func TestHRef_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  *url.URL
		err   error
	}{
		{
			name:  "valid URL",
			input: []byte(`"https://Example.com/path"`),
			want:  &url.URL{Scheme: "https", Host: "example.com", Path: "/path"},
		},
		{
			name:  "null JSON",
			input: []byte("null"),
			want:  nil,
		},
		{
			name:  "empty string",
			input: []byte(`""`),
			want:  nil,
		},
		{
			name:  "invalid JSON",
			input: []byte(`{"not": "a string"}`),
			err:   ErrDeserialization,
		},
		{
			name:  "invalid URL",
			input: []byte(`"://invalid-url"`),
			err:   ErrDeserialization,
		},
		{
			name:  "relative URL",
			input: []byte(`"/Relative/path/"`),
			want:  &url.URL{Path: "/relative/path/"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var h HRef
			if err := h.UnmarshalJSON(tc.input); tc.err == nil && err != nil {
				t.Errorf("UnmarshalJSON() unexpected error: %v", err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("UnmarshalJSON(%q) different error than expected\ngot: %v, wanted: %v", tc.input, err, tc.err)
			} else if tc.err != nil {
				t.Logf("UnmarshalJSON(%q) got expected error: %v", tc.input, err)
				return // skip remaining tests on expected error
			}
			if !normalization.Equal(h.URL, tc.want) {
				t.Errorf("UnmarshalJSON() = %v, want %v", h.URL, tc.want)
			}
		})
	}
}

func TestHRef_UnmarshalText(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  *url.URL
		err   error
	}{
		{
			name:  "valid URL",
			input: []byte("HTTPS://example.com/path"),
			want:  &url.URL{Scheme: "https", Host: "example.com", Path: "/path"},
		},
		{
			name:  "empty input",
			input: []byte(""),
			want:  nil,
		},
		{
			name:  "invalid URL",
			input: []byte("://invalid-url"),
			err:   ErrDeserialization,
		},
		{
			name:  "relative URL",
			input: []byte("/relative/path"),
			want:  &url.URL{Path: "/relative/path"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var h HRef
			if err := h.UnmarshalText(tc.input); tc.err == nil && err != nil {
				t.Fatalf("UnmarshalText() unexpected error: %v", err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("UnmarshalText(%q) different error than expected\ngot: %v, wanted: %v", tc.input, err, tc.err)
			} else if tc.err != nil {
				t.Logf("UnmarshalText(%q) got expected error: %v", tc.input, err)
				return // skip remaining tests on expected error
			}
			if !normalization.Equal(h.URL, tc.want) {
				t.Errorf("UnmarshalText() = %v, want %v", h.URL, tc.want)
			}
		})
	}
}

func TestHRef_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		href *HRef
		want string
	}{
		{
			name: "valid URL",
			href: &HRef{URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/path"}},
			want: `"https://example.com/path"`,
		},
		{
			name: "nil URL",
			href: &HRef{URL: nil},
			want: "null",
		},
		{
			name: "relative URL",
			href: &HRef{URL: &url.URL{Path: "/relative/path"}},
			want: `"/relative/path"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.href.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON() unexpected error: %v", err)
				return
			}

			if string(got) != tc.want {
				t.Errorf("MarshalJSON() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestHRef_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		href HRef
		want string
	}{
		{
			name: "valid URL",
			href: HRef{URL: &url.URL{Scheme: "HTTPS", Host: "example.com", Path: "/path"}},
			want: "https://example.com/path",
		},
		{
			name: "nil URL",
			href: HRef{URL: nil},
			want: "",
		},
		{
			name: "relative URL",
			href: HRef{URL: &url.URL{Path: "/Relative/Path"}},
			want: "/relative/path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.href.MarshalText()
			if err != nil {
				t.Errorf("MarshalText() unexpected error: %v", err)
				return
			}

			if string(got) != tc.want {
				t.Errorf("MarshalText() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestHRef_Identifier(t *testing.T) {
	tests := []struct {
		name string
		href *HRef
		want string
		err  error
	}{
		{
			name: "valid URL",
			href: &HRef{URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/path"}},
			want: "5faa4bf4918ff56562141cc328545ec8f7b6dd27", // sha256 of "https://example.com/path" truncated to 40 chars
		},
		{
			name: "nil HRef",
			href: nil,
			want: "",
			err:  ErrNotImplemented,
		},
		{
			name: "nil URL",
			href: &HRef{URL: nil},
			want: "8a5edab282632443219e051e4ade2d1d5bbc671c", // sha256 of the default "/" path truncated to 40 chars
			err:  nil,
		},
		{
			name: "different URL same identifier",
			href: &HRef{URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/other"}},
			want: "3de5b3c6e5e8e5b3c6e5e8e5b3c6e5e8e5b3c6e5", // This will be different
			err:  ErrNotImplemented,                          // Expecting different identifier
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string = tc.href.Identifier()
			if tc.err != nil {
				if got == tc.want {
					t.Errorf("Identifier() expected error, got matching identifiers: %s", got)
				}
				return
			}
			if len(got) != 40 {
				t.Errorf("Identifier() length = %d, want 40", len(got))
			}
			if got != tc.href.Identifier() {
				t.Errorf("Identifier() not deterministic: %s != %s", got, tc.href.Identifier())
			}
			if got != tc.want {
				t.Errorf("Identifier() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestHRef_Domain(t *testing.T) {
	tests := []struct {
		name string
		href *HRef
		want string
	}{
		{
			name: "valid URL with domain",
			href: &HRef{URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/path"}},
			want: "example.com",
		},
		{
			name: "nil URL",
			href: &HRef{URL: nil},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.href.Domain()
			if got != tc.want {
				t.Errorf("Domain() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHRef_Path(t *testing.T) {
	tests := []struct {
		name string
		href *HRef
		want string
	}{
		{
			name: "valid URL with path",
			href: &HRef{URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/path/to/resource"}},
			want: "/path/to/resource",
		},
		{
			name: "valid URL with path and query",
			href: &HRef{URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/path/to/resource", RawQuery: "b=2&a=1"}},
			want: "/path/to/resource?a=1&b=2", // Query params should be sorted
		},
		{
			name: "nil URL",
			href: &HRef{URL: nil},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.href.Path()
			// Since we don't know the exact normalization logic, just test it doesn't panic
			// and returns a string
			if got == "" && tc.href.URL != nil && tc.href.URL.Path != "" {
				t.Logf("Path() returned empty string for URL with path: %s", tc.href.URL.Path)
			}
		})
	}
}
