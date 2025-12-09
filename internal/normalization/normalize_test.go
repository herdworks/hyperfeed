package normalization

import (
	"errors"
	"net/url"
	"path/filepath"
	"testing"
)

func TestEqual(t *testing.T) {
	tests := []struct {
		a, b     string
		equality bool
	}{
		{"http://example.com", "http://example.com", true},
		{"http://example.com", "http://EXAMPLE.com", true},
		{"http://example.com:80", "http://example.com", true},
		{"https://example.com:443", "https://example.com", true},
		{"http://example.com:81", "http://example.com", false},
		{"https://example.com:80", "https://example.com", false},
		{"http://example.com/path?a=1&b=2", "http://example.com/path?b=2&a=1", true},
		{"http://example.com/path?a=1&b=2", "http://example.com/path?a=1&b=3", false},
		{"http://example.com/path", "http://example.com/other", false},
		{"http://example.com", "https://example.com", false},
		{"http://example.com", "http://example.org", false},
		{"http://example.com", "", false},
		{"", "", true},
	}
	for _, tc := range tests {
		a, err := url.Parse(tc.a)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.a, err)
		}
		b, err := url.Parse(tc.b)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.b, err)
		}
		if equality := Equal(a, b); equality != tc.equality {
			t.Errorf("Equal(%q, %q) = %v; want %v", tc.a, tc.b, equality, tc.equality)
		}
	}
}

func TestDomain(t *testing.T) {
	tests := []struct {
		href string
		want string // expected Host field
	}{
		{"https://www.Example.com:443/path?q=1", "example.com"},
		{"http://m.example.com/article", "example.com"},
		{"https://mobile.example.co.uk", "example.co.uk"},
		{"https://blog.example.com/", "blog.example.com"},
		{"https://example.com", "example.com"},
		{"http://WWW2.Example.ORG", "example.org"},
	}
	for _, tc := range tests {
		u, err := url.Parse(tc.href)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.href, err)
		}
		got := Domain(u)
		if got == "" {
			t.Fatalf("Domain(%q) = nil; want host %q", tc.href, tc.want)
		}
		if got != tc.want {
			t.Fatalf("Domain(%q) = %q; want %q", tc.href, got, tc.want)
		}
	}

	// No host → empty string
	u := &url.URL{Path: "/relative"}
	if got := Domain(u); got != "" {
		t.Fatalf("Domain(%v) = %v; want empty string", u, got)
	}

	// Nil URL → empty string
	if got := Domain(nil); got != "" {
		t.Fatalf("Domain(nil) = %v; want empty string", got)
	}
}

func TestIsCommonSubdomain(t *testing.T) {
	trues := []string{"www", "www1", "www2", "mobile", "m"}
	falses := []string{"blog", "news", "wWwX", "wm", "mob"}
	for _, s := range trues {
		if !isCommonSubdomain(s) {
			t.Fatalf("isCommonSubdomain(%q) = false; want true", s)
		}
	}
	for _, s := range falses {
		if isCommonSubdomain(s) {
			t.Fatalf("isCommonSubdomain(%q) = true; want false", s)
		}
	}
}

func TestSanitizeParams(t *testing.T) {
	in := "https://example.com/post?utm_source=x&utm_medium=y&utm_campaign=z&gclid=123&keep=ok"
	u, _ := url.Parse(in)
	got := SanitizeParams(u)
	if got.Query().Get("keep") != "ok" {
		t.Fatalf("keep param missing; got %v", got.RawQuery)
	}
	removed := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "gclid", "fbclid", "mc_cid", "mc_eid", "ref", "referrer"}
	for _, k := range removed {
		if got.Query().Has(k) {
			t.Fatalf("param %q not removed; got %v", k, got.RawQuery)
		}
	}
	// Should return a new pointer (pure function)
	if got == u {
		t.Fatalf("SanitizeParams should return a new pointer (not modify in place)")
	}
	// Original should be unchanged
	if u.RawQuery != "utm_source=x&utm_medium=y&utm_campaign=z&gclid=123&keep=ok" {
		t.Fatalf("original URL was mutated: %q", u.RawQuery)
	}

	// Idempotent: no tracking params present
	in2 := "https://example.com/path?a=1&b=2"
	u2, _ := url.Parse(in2)
	got2 := SanitizeParams(u2)
	if got2.RawQuery != "a=1&b=2" {
		t.Fatalf("unexpected query after sanitize: %q", got2.RawQuery)
	}
}

func TestURLNormalization(t *testing.T) {
	tests := []struct {
		href string
		want string // expected Host field
	}{
		{
			"HTTP://WWW.Example.com:80/a?b=2&a=1",
			"http://www.example.com/a?a=1&b=2",
		},
		{
			"http://www.example.com:81/a?a=1&b=2",
			"http://www.example.com:81/a?a=1&b=2",
		},
		{
			"https://EXAMPLE.com:443/x?z=9&y=8",
			"https://example.com/x?y=8&z=9",
		},
	}
	for _, tc := range tests {
		u, _ := url.Parse(tc.href)
		got := URL(u)
		if got.String() != tc.want {
			t.Fatalf("URL(%q) = %q; want %q", tc.href, got.String(), tc.want)
		}
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{"simple file", "cover.png", filepath.Join("cover.png"), nil},
		{"nested directories", "img/cover.png", filepath.Join("img", "cover.png"), nil},
		{"trim spaces", "  assets/banner.JPG  ", filepath.Join("assets", "banner.JPG"), nil},
		{"windows separators", "img\\cover.png", filepath.Join("img", "cover.png"), nil},
		{"leading dot segment", "./img/cover.png", filepath.Join("img", "cover.png"), nil},
		{"uppercase", "IMG/HERO.PNG", filepath.Join("IMG", "HERO.PNG"), nil},
		{"empty", "", "", ErrInvalidPath},
		{"only dot", ".", "", ErrInvalidPath},
		{"parent traversal", "../secret.txt", "", ErrInvalidPath},
		{"embedded traversal", "img/../secret.txt", filepath.Join("secret.txt"), nil},
		{"absolute", "/etc/passwd", "", ErrInvalidPath},
		{"invalid chars", "img/photo?.png", "", ErrInvalidPath},
		{"double slash", "img//cover.png", filepath.Join("img", "cover.png"), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var err error
			if got, err = CleanPath(tc.in); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("CleanPath(%q) different error than expected:\ngot: %v\nwanted: %v", tc.in, err, tc.err)
			}
			if got != tc.want {
				t.Fatalf("CleanPath(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIdentifierNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://example.com", "2a1b402420ef46577471cdc7409b0fa2c6a204db"},
		{"http://example.com/", "2a1b402420ef46577471cdc7409b0fa2c6a204db"},
		{"http://EXAMPLE.com", "2a1b402420ef46577471cdc7409b0fa2c6a204db"},
		{"http://example.com:80", "2a1b402420ef46577471cdc7409b0fa2c6a204db"},
		{"https://example.com/path", "5faa4bf4918ff56562141cc328545ec8f7b6dd27"},
		{"https://example.com:443/path", "5faa4bf4918ff56562141cc328545ec8f7b6dd27"},
		{"https://example.com/path?a=1&b=2", "94ab2087f4c4ce19d248cfbab9e2b19cc5065e44"},
		{"", "8a5edab282632443219e051e4ade2d1d5bbc671c"}, // the empty url identifier
	}
	for _, tc := range tests {
		t.Logf("normalize %q identifier\n", tc.input)
		u, err := url.Parse(tc.input)
		if err != nil {
			t.Fatalf("url %q parser error: %v", tc.input, err)
		}
		u = URL(u)
		t.Logf("normaled %q url\n", u.String())
		got := Identifier(u.String())
		if got != tc.want {
			t.Fatalf("normalized %q identifier error\n\twanted: %q\n\t   got: %q", tc.input, tc.want, got)
		} else {
			t.Logf("normalized %q identifier: %q", u.String(), got)
		}
	}
}
