package hyperfeed

import (
	"encoding/xml"
	"net/url"

	"github.com/herdworks/hypertemplates-go/opml"
)

type OPML struct {
	*opml.OPML

	URI *url.URL `json:"uri,omitempty" xml:"-"`
}

func ParseOPML(doc []byte) (opml *OPML, err error) {
	opml = &OPML{}
	err = opml.Unmarshal(doc)
	return opml, err
}

func (o *OPML) Load() (err error) {
	if o.OPML == nil {
		o.OPML = &opml.OPML{}
	}
	if o.URI != nil {
		o.OPML.URI = o.URI
	}
	return o.OPML.Load()
}

func (opml *OPML) Unmarshal(data []byte) error {
	return xml.Unmarshal(data, &opml.OPML)
}

func (opml *OPML) MarshalIndent(prefix string, indent string) ([]byte, error) {
	return xml.MarshalIndent(opml.OPML, prefix, indent)
}
