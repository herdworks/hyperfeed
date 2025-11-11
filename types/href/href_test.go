package href

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/herdworks/hyperfeed-go/internal/normalization"
)

func TestHref_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  *Href
		err   error
	}{
		{
			name:  "valid URL",
			input: []byte(`{ "url": "https://Example.com/path" }`),
			want:  &Href{Scheme: "https", Host: "Example.com", Path: "/path"},
		},
		{
			name:  "null URL",
			input: []byte(`{ "url": null }`),
			want:  nil,
		},
		{
			name:  "empty object",
			input: []byte(`{}`),
			want:  nil,
		},
		{
			name:  "empty string",
			input: []byte(`{ "url": "" }`),
			want:  &Href{},
		},
		{
			name:  "non URL string",
			input: []byte(`{ "url": "this is not a url" }`),
			want:  &Href{Path: "this is not a url", RawPath: "this is not a url"},
		},
		{
			name:  "invalid URL",
			input: []byte(`{ "url": "://invalid-url" }`),
			err:   ErrDeserialization,
		},
		{
			name:  "relative URL",
			input: []byte(`{ "url": "/Relative/path/" }`),
			want:  &Href{Path: "/Relative/path/"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			type wrapper struct {
				URL *Href `json:"url"`
			}
			var w wrapper
			err := json.Unmarshal(tc.input, &w)
			if tc.err != nil && errors.Is(err, tc.err) {
				return // expected error occurred
			} else if err != nil {
				t.Fatalf("unexpected error;\n\twanted: %v\n\tgot:    %v", tc.err, err)
			}
			if w.URL == nil && tc.want == nil {
				return // both nil, ok
			}
			if w.URL == nil && tc.want != nil {
				t.Fatalf("mismatched Hrefs;\n\twanted: %#v\n\tgot:   nil", tc.want)
			}
			if *tc.want != *w.URL {
				t.Fatalf("mismatched Hrefs;\n\twanted: %#v\n\tgot:    %#v", tc.want, w.URL)
			}
		})
	}
}

func TestHref_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		href *Href
		want string
	}{
		{
			name: "valid URL",
			href: &Href{Scheme: "HTTPS", Host: "example.com", Path: "/path"},
			want: `"HTTPS://example.com/path"`,
		},
		{
			name: "nil URL",
			href: nil,
			want: "null",
		},
		{
			name: "relative URL",
			href: &Href{Path: "/relative/Path"},
			want: `"/relative/Path"`,
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
				t.Errorf("MarshalJSON() error;\n\twanted: %s\n\tgot:    %s", tc.want, got)
			}
		})
	}
}

func TestHref_UnmarshalText(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  *Href
		err   error
	}{
		{
			name:  "valid URL",
			input: []byte("HTTPS://example.com/path"),
			want:  &Href{Scheme: "https", Host: "example.com", Path: "/path"},
		},
		{
			name:  "empty input",
			input: []byte(""),
			want:  &Href{},
		},
		{
			name:  "invalid URL",
			input: []byte("://invalid-url"),
			err:   ErrDeserialization,
		},
		{
			name:  "relative URL",
			input: []byte("/relative/path"),
			want:  &Href{Path: "/relative/path"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var h *Href = &Href{}
			var err error
			if err = h.UnmarshalText(tc.input); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Fatalf("unexpected error;\n\twanted: %v\n\tgot:    %v", tc.err, err)
			}
			if *h != *tc.want {
				t.Fatalf("mismatched Hrefs;\n\twanted: %#v\n\tgot:    %#v", tc.want, h)
			}
		})
	}
}

func TestHref_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		href *Href
		want string
	}{
		{
			name: "valid URL",
			href: &Href{Scheme: "HTTPS", Host: "example.com", Path: "/path"},
			want: "HTTPS://example.com/path",
		},
		{
			name: "nil URL",
			href: nil,
			want: "",
		},
		{
			name: "relative URL",
			href: &Href{Path: "/Relative/Path"},
			want: "/Relative/Path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bytes []byte
			var err error
			if bytes, err = tc.href.MarshalText(); err != nil {
				t.Errorf("MarshalText() unexpected error: %v", err)
				return
			}

			if string(bytes) != tc.want {
				t.Errorf("MarshalText() err\n\twanted: %s, \n\tgot:   %s", tc.want, bytes)
			}
		})
	}
}

func TestHrefURLs(t *testing.T) {
	tests := []struct {
		name string
		href *Href
		url  *url.URL
	}{
		{
			name: "valid absolute URL",
			href: &Href{Scheme: "https", Host: "example.com", Path: "/path"},
			url:  &url.URL{Scheme: "https", Host: "example.com", Path: "/path"},
		},
		{
			name: "valid relative URL",
			href: &Href{Path: "/relative/path"},
			url:  &url.URL{Path: "/relative/path"},
		},
		{
			name: "nil Href",
			href: nil,
			url:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !normalization.Equal(tc.href.URL(), tc.url) {
				t.Errorf("Href.URL() = %v, want %v", tc.href.URL(), tc.url)
			}
		})
	}
}
