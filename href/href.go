package href

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// ============================ Package Types ==================================

// Href is a thin wrapper around *url.URL that marshals to/from a JSON string
// and implements TextMarshaler/TextUnmarshaler for broader codec support.
type Href url.URL

// ============================ Package Variables ==============================
var (
	ErrDeserialization = errors.New("href deserialization error")
	ErrSerialization   = errors.New("href serialization error")
)

// ============================ Href Methods ===================================

// UnmarshalJSON parses a JSON string into an Href. "null" and "" become nil.
func (href *Href) UnmarshalJSON(b []byte) (err error) {
	var s string
	var u *url.URL
	if string(b) == "null" {
		return nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return errors.Join(ErrDeserialization, err)
	}
	if s == "" {
		*href = Href{}
		return nil
	}
	if u, err = url.Parse(s); err != nil {
		return errors.Join(ErrDeserialization, err)
	}
	*href = Href(*u)
	return nil
}

// MarshalJSON renders the URL as a JSON string or null when unset.
func (href *Href) MarshalJSON() (bytes []byte, err error) {
	if href == nil {
		return []byte("null"), nil
	}
	if bytes, err = json.Marshal(href.URL().String()); err != nil {
		err = errors.Join(ErrSerialization, err)
	}
	return bytes, err
}

// MarshalText renders the URL as bytes; empty when unset.
func (href *Href) MarshalText() ([]byte, error) {
	if href == nil {
		return []byte{}, nil
	}
	u := url.URL(*href)
	return []byte(u.String()), nil
}

// UnmarshalText parses a URL from text; empty becomes zero value Href.
func (href *Href) UnmarshalText(text []byte) (err error) {
	var u *url.URL
	if len(text) == 0 {
		*href = Href{}
		return nil
	}
	if u, err = url.Parse(string(text)); err != nil {
		return errors.Join(ErrDeserialization, err)
	}
	*href = Href(*u)
	return nil
}

// URL returns the underlying *url.URL
func (href *Href) URL() *url.URL {
	if href == nil {
		return nil
	}
	u := url.URL(*href)
	return &u
}

// RedactedURL returns a redacted version of the URL with sensitive info removed
//
// Sensitive info includes:
// * User info (username, password)
// * Fragments
// * Sensitive query parameters
func (href *Href) RedactedURL() *url.URL {
	var values url.Values = url.Values{}
	if href == nil {
		return nil
	}
	u := href.URL()
	u.User = nil
	for key := range u.Query() {
		if RedactedURLValue(key) {
			continue
		}
		values.Add(key, u.Query().Get(key))
	}
	u.RawQuery = values.Encode()
	u.ForceQuery = false
	u.Fragment = ""
	return u
}

// IsAbs reports whether the Href is absolute
func (href *Href) IsAbs() bool {
	if href == nil {
		return false
	}
	return href.URL().IsAbs()
}

// ResolveReference resolves a reference URL against this Href
func (href *Href) ResolveReference(in *Href) (out *Href) {
	if in == nil {
		return nil
	}
	if href == nil {
		return in
	}
	inURL := in.URL()
	hrefURL := href.URL()
	resolvedURL := hrefURL.ResolveReference(inURL)
	return (*Href)(resolvedURL)
}

// String returns the string representation of the Href
func (href *Href) String() string {
	if href == nil {
		return ""
	}
	return href.URL().String()
}

func (href *Href) SameOrigin(in any) bool {
	if href == nil || href.URL() == nil || href.URL().Host == "" {
		return false // href does not have a valid host, so in cannot be same-origin
	}
	ref, err := Parse(in)
	if err != nil || ref == nil || ref.URL() == nil || ref.URL().Host == "" {
		return false // ref does not have a valid host, so cannot be same-origin
	}
	return href.URL().Host == ref.URL().Host // true if href and in have the same host
}

// ============================ Helper Functions ===============================
// Parse parses a string into the Href
func Parse(in any) (out *Href, err error) {
	var u *url.URL
	if in == nil {
		return nil, fmt.Errorf("cannot parse nil as href")
	}
	switch v := in.(type) {
	case Href:
		return &v, nil
	case *Href:
		return v, nil
	case url.URL:
		u = &v
	case *url.URL:
		u = v
	case string:
		u, err = url.Parse(v)
	case *string:
		u, err = url.Parse(*v)
	case []byte:
		u, err = url.Parse(string(v))
	default:
		err = fmt.Errorf("unsupported type for href parsing: %T", in)
	}
	return (*Href)(u), err
}

func ResolveHrefString(base, ref any, w io.Writer) string {
	if href := ResolveHref(base, ref, w); href != nil {
		return href.String()
	}
	return ""
}

func ResolveHref(base, ref any, w io.Writer) *Href {
	var b, r *Href
	var err error
	if b, err = Parse(base); err != nil || b == nil {
		fmt.Fprintln(w, err)
		return nil
	}
	if r, err = Parse(ref); err != nil || r == nil {
		fmt.Fprintln(w, err)
		return nil
	}
	return b.ResolveReference(r)
}

func RedactedURLValue(key string) bool {
	var redacted = []string{"token", "auth", "key", "password", "member"}
	for _, redact := range redacted {
		if strings.Contains(strings.ToLower(key), redact) {
			return true
		}
	}
	return false
}
