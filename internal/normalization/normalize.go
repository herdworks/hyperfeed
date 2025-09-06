package normalization

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// URL returns a canonicalized copy of u:
// - Lowercases scheme and host
// - Removes default ports (http:80, https:443)
// - Sorts query parameters (by key and value)
// - Returns nil if u is nil.
func URL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	v := *u // shallow copy
	if v.Scheme != "" {
		v.Scheme = strings.ToLower(v.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (v.Scheme == "http" && port == "80") || (v.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		v.Host = net.JoinHostPort(host, port)
	} else {
		v.Host = host
	}
	v.RawQuery = v.Query().Encode() // Encode sorts keys
	return &v
}

// Domain returns a URL containing a sanitized domain
//
// Used for feed identification.
func Domain(u *url.URL) *url.URL {
	if u == nil || u.Hostname() == "" {
		return nil
	}
	u = URL(u)
	labels := strings.Split(u.Hostname(), ".")
	if len(labels) > 1 && isCommonSubdomain(labels[0]) {
		labels = labels[1:]
	}
	domain := strings.Join(labels, ".")
	return &url.URL{Host: domain}
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
func Path(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u = URL(u)
	u = SanitizeParams(u)
	return &url.URL{Path: u.Path, RawQuery: u.RawQuery}
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

// HexID
func HexID(u *url.URL) string {
	u = Path(u)
	sum := sha256.Sum256([]byte(u.String()))
	hexFull := hex.EncodeToString(sum[:])
	id := hexFull[:40]
	return id
}

// String returns a normalized string suitable for comparisons. Currently passthrough.
func String(s string) string { return s }

// GUID returns a canonicalized GUID/ID string. Currently passthrough.
func GUID(id string) string { return id }
