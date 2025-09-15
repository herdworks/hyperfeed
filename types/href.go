package hyperfeed

import (
	"encoding/json"
	"fmt"
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
)

// HRef is a thin wrapper around *url.URL that marshals to/from a JSON string
// and implements TextMarshaler/TextUnmarshaler for broader codec support.
type HRef struct {
	*url.URL
}

// UnmarshalJSON parses a JSON string into a URL. "null" and "" become nil.
func (href *HRef) UnmarshalJSON(b []byte) (err error) {
	var s string
	var u *url.URL
	if string(b) == "null" {
		href.URL = nil
		return nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("href must be a JSON string: %w", err)
	}
	if s == "" {
		href.URL = nil
		return nil
	}
	if u, err = url.Parse(s); err != nil {
		return fmt.Errorf("invalid href: %w", err)
	}
	href.URL = normalized.URL(u)
	return nil
}

// MarshalJSON renders the URL as a JSON string or null when unset.
func (href HRef) MarshalJSON() ([]byte, error) {
	if href.URL == nil {
		return []byte("null"), nil
	}
	return json.Marshal(normalized.URL(href.URL).String())
}

// MarshalText renders the URL as bytes; empty when unset.
func (href HRef) MarshalText() ([]byte, error) {
	if href.URL == nil {
		return []byte{}, nil
	}
	return []byte(normalized.URL(href.URL).String()), nil
}

// UnmarshalText parses a URL from text; empty becomes nil.
func (href *HRef) UnmarshalText(text []byte) (err error) {
	var u *url.URL
	if len(text) == 0 {
		href.URL = nil
		return nil
	}
	if u, err = url.Parse(string(text)); err != nil {
		return err
	}
	href.URL = normalized.URL(u)
	return nil
}

// Identifier represents a stable identifier for a feed or item.
func (href *HRef) Identifier() string {
	if href == nil {
		href = &HRef{}
	}
	if href.URL == nil {
		href.URL = &url.URL{}
	}
	u := normalized.URL(href.URL)
	return normalized.Identifier([]byte(u.String()))
}

// Domain returns the HyperFeed-normalized domain for the URL.
func (href *HRef) Domain() string {
	return normalized.Domain(href.URL)
}

// Path returns the HyperFeed-normalized path for the URL.
func (href *HRef) Path() string {
	return normalized.Path(href.URL)
}
