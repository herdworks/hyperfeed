package normalization

import "net/url"

// URL returns a canonicalized copy of u. For now, it returns u unchanged.
// Implementation to be filled in (lowercasing host, removing default ports,
// sorting query params where safe, etc.).
func URL(u *url.URL) *url.URL {
	return u
}

// String returns a normalized string suitable for comparisons (e.g., trimming
// whitespace, collapsing internal spaces). Currently a passthrough.
func String(s string) string { return s }

// GUID returns a canonicalized GUID/ID string (e.g., trimming, lowercasing
// when appropriate). Currently a passthrough.
func GUID(id string) string { return id }
