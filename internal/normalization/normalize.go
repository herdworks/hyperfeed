package normalization

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"path/filepath"
)

// ============================ Package Variables ============================

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrInvalidPath    = errors.New("invalid path")
)

var CleanPathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var CleanPathReservedFileNames = []string{
	"cache.lock",
}

// ============================ Helper Functions ==============================

// URL returns a canonicalized copy of u:
// - Returns nil if incoming url is nil.
// - Lowercases scheme and host
// - Removes default ports (http:80, https:443)
// - Sorts query parameters (by key and value)
// - Removes trailing slash from path (except for root "/")
func URL(incoming *url.URL) *url.URL {
	if incoming == nil {
		return &url.URL{}
	}
	normalized := *incoming // shallow copy
	if normalized.Scheme != "" {
		normalized.Scheme = strings.ToLower(normalized.Scheme)
	}
	if (normalized.Scheme == "http" && normalized.Port() == "80") || (normalized.Scheme == "https" && normalized.Port() == "443") {
		normalized.Host = normalized.Hostname() // remove default HTTP/HTTPS ports
	}
	if normalized.Host != "" {
		normalized.Host = strings.ToLower(normalized.Host)            // lowercase host
		normalized.Host = strings.TrimPrefix(normalized.Host, "www.") // remove "www." prefix
	}
	if normalized.Path == "" {
		normalized.Path = "/" // ensure non-empty path
	} else {
		normalized.Path = strings.ToLower(normalized.Path)
	}
	if normalized.Path != "/" && strings.HasSuffix(normalized.Path, "/") {
		normalized.Path = strings.TrimSuffix(normalized.Path, "/") // remove trailing slash except for root
	}
	normalized.RawQuery = normalized.Query().Encode() // Encode sorts keys
	return &normalized
}

// Href returns a normalized Href
func Href(incoming string) (href string, err error) {
	var u *url.URL
	if u, err = url.Parse(incoming); err != nil {
		return "", err
	}
	u = URL(u)
	return u.String(), nil
}

// Equal reports whether two URLs are equivalent after normalization.
func Equal(a, b *url.URL) bool {
	A := URL(a)
	B := URL(b)
	if A == nil && B == nil {
		return true
	}
	if A == nil || B == nil {
		return false
	}
	return A.String() == B.String()
}

// Domain returns a sanitized domain string
//
// Used for feed identification.
func Domain(u *url.URL) string {
	if u == nil || u.Hostname() == "" {
		return ""
	}
	u = URL(u)
	labels := strings.Split(u.Hostname(), ".")
	if len(labels) > 1 && isCommonSubdomain(labels[0]) {
		labels = labels[1:]
	}
	return strings.Join(labels, ".")
}

// isCommonSubdomain returns true if s matches a common subdomain pattern like "www"
func isCommonSubdomain(s string) bool {
	var patterns []*regexp.Regexp
	patterns = append(patterns, regexp.MustCompile(`^www\d*$`)) // Matches "www", "www1", "www2", etc.
	patterns = append(patterns, regexp.MustCompile(`^mobile$`)) // Matches "mobile"
	patterns = append(patterns, regexp.MustCompile(`^m$`))      // Matches "m"
	for _, p := range patterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// Path returns a URL containing a sanitized path + query (no scheme/host).
func Path(u *url.URL) string {
	if u == nil {
		return ""
	}
	u = URL(u)
	u = SanitizeParams(u)
	p := url.URL{Path: u.Path, RawQuery: u.RawQuery}
	return p.String()
}

// SanitizeParams removes tracking and secret query parameters from a URL.
func SanitizeParams(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	v := *URL(u) // normalize first, then copy
	if v.RawQuery == "" {
		return &v
	}
	trackers := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"gclid", "fbclid", "mc_cid", "mc_eid", "ref", "referrer",
	}
	q := v.Query()
	for _, t := range trackers {
		q.Del(t)
	}
	v.RawQuery = q.Encode()
	return &v
}

// Identifier returns a HyperFeed identifier (first 40 chars of SHA256 hex)
//
// Inspired by Git's SHA-256-based object naming.
// https://git-scm.com/docs/hash-function-transition
func Identifier(incoming string) string {
	incoming = strings.ToLower(incoming)
	sum := sha256.Sum256([]byte(incoming))
	return hex.EncodeToString(sum[:])[:40]
}

// CleanPath validates and normalizes relative asset paths.
//
// The returned path is safe to join against a cache namespace because:
//   - Empty paths are rejected
//   - Absolute paths are rejected
//   - Path is cleaned, but resulting segments must stay within the item-scoped root
//   - Cleaned paths must not collide with reserved filenames (e.g. `cache.lock`)
//   - Each segment must match [A-Za-z0-9._-]+
//
// New path separators use the current OS via filepath.Join.
func CleanPath(p string, reserved ...string) (string, error) {
	var clean string
	var segments []string
	p = strings.TrimSpace(p)             // trim leading/trailing whitespace
	p = strings.ReplaceAll(p, "\\", "/") // Treat Windows separators like POSIX for normalization.
	if p == "" {
		return "", ErrInvalidPath
	}
	if !utf8.ValidString(p) {
		return "", ErrInvalidPath
	}
	if clean = path.Clean(p); clean == "." {
		return "", ErrInvalidPath
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", ErrInvalidPath
	}
	if strings.HasPrefix(clean, "/") {
		return "", ErrInvalidPath
	}
	if len(reserved) == 0 {
		reserved = append(reserved, CleanPathReservedFileNames...) // copy default reserved names
	}
	if slices.Contains(reserved, clean) {
		return "", ErrInvalidPath
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", ErrInvalidPath
		}
		if !CleanPathSegmentPattern.MatchString(segment) {
			return "", ErrInvalidPath
		}
		segments = append(segments, segment)
	}
	return filepath.Join(segments...), nil
}
