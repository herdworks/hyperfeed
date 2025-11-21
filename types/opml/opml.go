package opml

import (
	"encoding/xml"
	"net/url"

	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ============================ Package Types =================================

// OPML Types
// Spec: https://opml.org/spec2.opml
type OPML struct {
	URI *url.URL `json:"-" yaml:"-"`

	XMLName xml.Name `xml:"opml" json:"opml"`
	Head    Head     `xml:"head" json:"head"`
	Body    Body     `xml:"body" json:"body"`
	Version string   `xml:"version,attr,omitempty" json:"version,omitempty"`
}

type Head struct {
	Title        string `xml:"title,omitempty" json:"title,omitempty"`
	OwnerId      string `xml:"ownerId,omitempty" json:"owner_id,omitempty"`
	OwnerName    string `xml:"ownerName,omitempty" json:"owner_name,omitempty"`
	OwnerEmail   string `xml:"ownerEmail,omitempty" json:"owner_email,omitempty"`
	DateCreated  string `xml:"dateCreated,omitempty" json:"date_created,omitempty"`
	DateModified string `xml:"dateModified,omitempty" json:"date_modified,omitempty"`
}

type Body struct {
	Outlines Outlines `xml:"outline" json:"outlines"`
}

type Outline struct {
	Text         string    `xml:"text,attr" json:"text,omitempty"`
	Type         string    `xml:"type,attr,omitempty" json:"type,omitempty"`
	Title        string    `xml:"title,attr,omitempty" json:"title,omitempty"`
	Description  string    `xml:"description,attr,omitempty" json:"description,omitempty"`
	HTMLURL      string    `xml:"htmlUrl,attr,omitempty" json:"htmlUrl,omitempty"` // e.g. https://hypertemplates.net/
	XMLURL       string    `xml:"xmlUrl,attr,omitempty" json:"xmlUrl,omitempty"`   // e.g. https://hypertemplates.net/atom.xml
	Created      string    `xml:"created,attr,omitempty" json:"created,omitempty"`
	Category     string    `xml:"category,attr,omitempty" json:"category,omitempty"`
	Version      string    `xml:"version,attr,omitempty" json:"version,omitempty"`
	Outlines     []Outline `xml:"outline" json:"outlines,omitempty"`
	IsComment    bool      `xml:"isComment,attr,omitempty" json:"isComment,omitempty"`
	IsBreakpoint bool      `xml:"isBreakpoint,attr,omitempty" json:"isBreakpoint,omitempty"`

	// custom attributes
	FaviconURL string `xml:"faviconUrl,attr,omitempty" json:"faviconUrl,omitempty"`
	AudioURL   string `xml:"audioUrl,attr,omitempty" json:"audioUrl,omitempty"` // e.g. podcast URL
	VideoURL   string `xml:"videoUrl,attr,omitempty" json:"videoUrl,omitempty"` // e.g. vlog URL
}
type Outlines []*Outline

// ============================ OPML Methods ==================================

// opml.Load()
//
// Read OPML file from disk and unmarshal it into an OPML struct.
//
// Usage:
//
//	var opml *OPML
//	var err error
//	if opml.URI, err = url.Parse("path/to/opml/file.opml"); err != nil { return err }
//	if err = opml.Load(); err != nil { return err }
func (opml *OPML) Load() error {
	data, err := os.ReadFile(opml.URI.Path)
	if err != nil {
		return err
	}
	return opml.Unmarshal(data)
}

func (opml *OPML) Validate() error {
	for _, outline := range opml.Body.Outlines {
		if err := outline.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (opml *OPML) Unmarshal(data []byte) error {
	return xml.Unmarshal(data, &opml)
}

func (opml *OPML) MarshalIndent(prefix string, indent string) ([]byte, error) {
	if opml.Version == "" {
		opml.Version = "2.0"
	}
	return xml.MarshalIndent(opml, prefix, indent)
}

func (opml *OPML) ToJSON() (jsonBytes []byte, err error) {
	return json.Marshal(opml)
}

func (opml *OPML) ToJSONIndented(prefix string, indent string) ([]byte, error) {
	return json.MarshalIndent(opml, prefix, indent)
}

func (opml *OPML) DateCreated() (time.Time, error) {
	return parseDate(opml.Head.DateCreated)
}
func (opml *OPML) DateModified() (time.Time, error) {
	return parseDate(opml.Head.DateModified)
}

func (opml *OPML) AddOutline(outline *Outline) (err error) {
	if err = outline.Validate(); err != nil {
		return err
	}
	opml.Body.Outlines = append(opml.Body.Outlines, outline)
	return nil
}
func (opml *OPML) AddSubscription(outline *Outline) (err error) {
	outline.Type = "rss"
	outline.Version = "RSS"
	return opml.AddOutline(outline)
}

// ============================ Outline Methods ===============================

func (outline *Outline) Validate() error {
	if outline.Type == "" {
		return fmt.Errorf("outline.type is required")
	}
	if outline.Text == "" {
		return fmt.Errorf("outline.text is required")
	}
	if outline.XMLURL == "" {
		return fmt.Errorf("outline.xmlUrl is required")
	}
	return nil
}

// ============================ Helper Functions ==============================

func ParseOPML(doc []byte) (opml *OPML, err error) {
	opml = &OPML{}
	err = opml.Unmarshal(doc)
	return opml, err
}

// parseDate
//
// parses RFC822 date strings
func parseDate(date string) (ts time.Time, err error) {
	var now = time.Now()
	var layouts []string = []string{
		// RFC822 with zone name, e.g. "Mon, 02 Jan 06 15:04 MST"
		"Mon, 02 Jan 06 15:04 MST",
		// RFC822 without weekday, zone name, e.g. "02 Jan 06 15:04 MST"
		"02 Jan 06 15:04 MST",
		// RFC822Z with numeric offset, e.g. "Mon, 02 Jan 06 15:04 -0700"
		"Mon, 02 Jan 06 15:04 -0700",
		// RFC822Z without weekday, numeric offset, e.g. "02 Jan 06 15:04 -0700"
		"02 Jan 06 15:04 -0700",
		// RFC1123 with seconds and zone name, e.g. "Mon, 02 Jan 2006 15:04:05 MST"
		"Mon, 02 Jan 2006 15:04:05 MST",
		// RFC1123 without weekday, with seconds, zone name
		"02 Jan 2006 15:04:05 MST",
		// RFC1123Z with numeric offset and seconds
		"Mon, 02 Jan 2006 15:04:05 -0700",
		// RFC1123Z without weekday, with seconds, numeric offset
		"02 Jan 2006 15:04:05 -0700",
	}
	if date == "" {
		return now, nil
	}
	for _, layout := range layouts {
		if ts, err = time.Parse(layout, date); err == nil {
			return ts, nil
		}
	}
	return now, fmt.Errorf("failed to parse date: %s", date)
}
