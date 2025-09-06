package normalization

import (
	"net/url"
	"testing"
)

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
		if got == nil {
			t.Fatalf("Domain(%q) = nil; want host %q", tc.href, tc.want)
		}
		if got.Host != tc.want {
			t.Fatalf("Domain(%q).Host = %q; want %q", tc.href, got.Host, tc.want)
		}
		if got.Scheme != "" || got.Path != "" || got.RawQuery != "" {
			t.Fatalf("Domain(%q) should clear scheme/path/query; got %v", tc.href, got)
		}
	}

	// No host → nil
	u := &url.URL{Path: "/relative"}
	if got := Domain(u); got != nil {
		t.Fatalf("Domain(%v) = %v; want nil", u, got)
	}

	// Nil URL → nil
	if got := Domain(nil); got != nil {
		t.Fatalf("Domain(nil) = %v; want nil", got)
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

func TestStringGUIDPassthrough(t *testing.T) {
	if String("  x  ") != "  x  " {
		t.Fatal("String should be passthrough")
	}
	if GUID("ID-123") != "ID-123" {
		t.Fatal("GUID should be passthrough")
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
