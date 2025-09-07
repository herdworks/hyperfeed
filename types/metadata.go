package hyperfeed

import "encoding/xml"

// Metadata represents a normalized bag of properties for non-standard fields.
type Metadata map[string]Property

type Property struct {
	Namespace string
	Name      string
	Value     string
	RawValue  []byte
}

// Extension preserves any XML element outside the core spec during parsing.
// It captures the qualified name, attributes, character data, and inner XML.
type Extension struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `json:"-"`
	Text     string     `json:"-"`
	InnerXML string     `json:"-"`
}

// UnmarshalXML captures attributes, text, and raw inner XML for generic elements.
func (e *Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	e.XMLName = start.Name
	e.Attrs = append(e.Attrs[:0], start.Attr...)
	var w struct {
		Text  string `xml:",chardata"`
		Inner string `xml:",innerxml"`
	}
	if err := d.DecodeElement(&w, &start); err != nil {
		return err
	}
	e.Text = w.Text
	e.InnerXML = w.Inner
	return nil
}

// ExtensionNamespace captures a single xmlns prefix -> URI mapping.
type ExtensionNamespace struct {
	Prefix string `json:"prefix,omitempty"`
	URI    string `json:"uri,omitempty"`
}
