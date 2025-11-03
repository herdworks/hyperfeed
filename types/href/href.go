package hyperfeed

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	fmt.Println(u.String())
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

// String returns the string representation of the Href
func (href *Href) String() string {
	if href == nil {
		return ""
	}
	return href.URL().String()
}

// ============================ Helper Functions ===============================
// Parse parses a string into the Href
func Parse(s string) (href *Href, err error) {
	var u *url.URL
	if u, err = url.Parse(s); err != nil {
		return nil, errors.Join(ErrDeserialization, err)
	}
	h := Href(*u)
	return &h, nil
}
