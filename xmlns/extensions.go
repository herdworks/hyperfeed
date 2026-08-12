package xmlns

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// TYPES
// =============================================================================

// Extension preserves any XML element outside the core spec during parsing.
// It captures the qualified name, attributes, character data, and inner XML.
type Extension struct {
	XMLName    xml.Name
	Attrs      []xml.Attr `json:"-"`
	Text       string     `json:"-"`
	InnerXML   string     `json:"-"`
	Extensions Extensions `xml:",any" json:"-"` // nested elements
}
type Extensions []Extension

// METHODS
// =============================================================================

// UnmarshalXML captures attributes, text, inner XML, and any nested extensions.
func (e *Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var payload struct {
		Text  string      `xml:",chardata"`
		Inner string      `xml:",innerxml"`
		Any   []Extension `xml:",any"`
	}
	if err := d.DecodeElement(&payload, &start); err != nil {
		return err
	}
	e.XMLName = start.Name
	e.Attrs = append(e.Attrs[:0], start.Attr...)
	e.Text = payload.Text
	e.InnerXML = payload.Inner
	e.Extensions = append(e.Extensions[:0], payload.Any...)
	return nil
}

func (e *Extension) HasAttribute(name string) (exists bool) {
	for _, attr := range e.Attrs {
		if strings.EqualFold(attr.Name.Local, name) {
			return true
		}
	}
	return false
}

func (e *Extension) GetAttribute(name string) (value string) {
	for _, attr := range e.Attrs {
		if strings.EqualFold(attr.Name.Local, name) {
			return attr.Value
		}
	}
	return ""
}

func (e *Extension) GetAttributeInt(name string) (int, error) {
	return strconv.Atoi(e.GetAttribute(name))
}

// check if extension property exists
// where len(keypath) == 0, returns "flat" lookup of matching namespace elements
// where len(keypath) == 1, returns "flat" lookup of matching namespace:key element
// where len(keypath) > 1, returns "nested" lookup
func (extensions Extensions) HasProperty(namespace string, keypath ...string) bool {
	for _, ext := range extensions {
		if !strings.EqualFold(ext.XMLName.Space, namespace) {
			continue // non-matching namespace
		}
		if len(keypath) == 0 {
			return true // namespace matches, and no key provided
		}
		if !strings.EqualFold(ext.XMLName.Local, keypath[0]) {
			continue // namespace matches, but key does not
		}
		if len(keypath) == 1 {
			return true // namespace & keypath matches
		}
		return ext.Extensions.HasProperty(namespace, keypath[1:]...) // nested lookup
	}
	return false
}

// get extension properties
// where len(keypath) == 1, returns "flat" lookup of matching extensions element
// where len(keypath) > 1, returns "nested" lookup of extensions element.Extensions
func (extensions Extensions) GetProperty(namespace string, keypath ...string) (extension *Extension) {
	if len(keypath) == 0 {
		return nil
	}
	for _, ext := range extensions {
		if strings.EqualFold(ext.XMLName.Space, namespace) && strings.EqualFold(ext.XMLName.Local, keypath[0]) {
			if len(keypath[1:]) == 0 {
				return &ext
			} else {
				return ext.Extensions.GetProperty(namespace, keypath[1:]...)
			}
		}
	}
	return nil
}

func (extensions Extensions) GetPropertyString(namespace string, name string) string {
	if extension := extensions.GetProperty(namespace, name); extension != nil {
		return extension.Text
	} else {
		return ""
	}
}
