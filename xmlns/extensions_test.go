package xmlns

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestExtensionMarshalXML(t *testing.T) {
	document := struct {
		XMLName xml.Name   `xml:"rss"`
		Itunes  string     `xml:"xmlns:itunes,attr"`
		Values  Extensions `xml:",any"`
	}{
		Itunes: ItunesPodcastNS,
		Values: Extensions{{
			XMLName: xml.Name{Space: ItunesPodcastNS, Local: "owner"},
			Extensions: Extensions{
				{XMLName: xml.Name{Space: ItunesPodcastNS, Local: "name"}, Text: "Example"},
				{XMLName: xml.Name{Space: ItunesPodcastNS, Local: "email"}, Text: "example@example.com"},
			},
		}, {
			XMLName: xml.Name{Space: "https://example.com/ns", Local: "custom"},
			Attrs:   []xml.Attr{{Name: xml.Name{Local: "kind"}, Value: "example"}},
			Text:    "Value",
		}},
	}
	data, err := xml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	for _, expected := range []string{
		`<itunes:owner>`,
		`<itunes:name>Example</itunes:name>`,
		`<itunes:email>example@example.com</itunes:email>`,
		`<custom xmlns="https://example.com/ns" kind="example">Value</custom>`,
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("extension XML missing %s: %s", expected, output)
		}
	}
}
