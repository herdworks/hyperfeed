package atom

import (
	"encoding/xml"
	"net/url"
	"regexp"
	"strings"
)

// This file is a derivative work of "golang.org/x/tools/blog/atom" and is
// subject to the same copyright and license terms.
//
// golang.org/x/tools/LICENSE
// Copyright 2009 The Go Authors.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
//      notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
//      copyright notice, this list of conditions and the following disclaimer
//      in the documentation and/or other materials provided with the
//      distribution.
//    * Neither the name of Google LLC nor the names of its
//      contributors may be used to endorse or promote products derived from
//      this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Adapted from encoding/xml/read_test.go.

// TYPES
// =============================================================================

// Type Atom defines XML data structures for an Atom feed.
//
// reference: https://www.ietf.org/rfc/rfc4287.txt
type Atom struct {
	XMLName  xml.Name  `xml:"http://www.w3.org/2005/Atom feed"`
	XMLNS    string    `xml:"xmlns:hypergraph,attr"`
	Title    string    `xml:"title"`
	Subtitle string    `xml:"subtitle,omitempty"`
	ID       string    `xml:"id"`
	Links    []Link    `xml:"link"`
	Authors  []*Author `xml:"author,omitempty"`
	Entries  []*Entry  `xml:"entry"`
	Updated  TimeStamp `xml:"updated"`

	Origin *url.URL `xml:"-" json:"-"`
}

type Entry struct {
	Title        string    `xml:"title"`
	ID           string    `xml:"id"`
	Links        []Link    `xml:"link"`
	Published    TimeStamp `xml:"published"`
	Updated      TimeStamp `xml:"updated"`
	Authors      []*Author `xml:"author"`
	Contributors []*Author `xml:"contributor"` // extension from golang.org/x/tools/blog/atom
	Summary      *Text     `xml:"summary"`
	Content      *Text     `xml:"content"`
}

type Link struct {
	Rel      string `xml:"rel,attr,omitempty"`
	Href     string `xml:"href,attr"`
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   uint   `xml:"length,attr,omitempty"`
}

type Author struct {
	Name     string `xml:"name"`
	URI      string `xml:"uri,omitempty"`
	Username string `xml:"hypergraph:username,omitempty"`
	Favicon  string `xml:"hypergraph:favicon,omitempty"`
	Email    string `xml:"email,omitempty"`
	InnerXML string `xml:",innerxml"`
}

type Text struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type TimeStamp = string

// VARIABLES
// =============================================================================
var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

// METHODS
// =============================================================================
func (atom *Atom) Render() (string, error) {
	atomBytes, err := xml.MarshalIndent(atom, "", "\t")
	if err != nil {
		return "", err
	}
	return string(atomBytes), nil
}

// Atom.Link()
func (atom *Atom) Link(rel string) *Link {
	for _, l := range atom.Links {
		if l.Rel == rel {
			return &l
		}
	}
	return nil
}

// Entry.Link()
func (entry *Entry) Link(rel string) *Link {
	for _, l := range entry.Links {
		if l.Rel == rel {
			return &l
		}
		if l.Rel == "" && rel == "alternate" {
			return &l
		}
	}
	return nil
}

// Entry.InternalLink()
// returns the first link whose host matches the base URL host
func (entry *Entry) InternalLink(base *url.URL) *Link {
	for _, l := range entry.Links {
		if l.URL() != nil && strings.TrimPrefix(l.URL().Host, "www.") == strings.TrimPrefix(base.Host, "www.") {
			return &l
		}
	}
	for _, l := range entry.Links {
		if l.Rel == "alternate" || l.Rel == "" {
			return &l // fallback to alternate/empty rel
		}
	}
	return &entry.Links[0] // fallback to first link? (this should not happen)
}

// Entry.ExternalLinks()
// returns all links whose host differs from the base URL host
func (entry *Entry) ExternalLinks(base *url.URL) (links []*Link) {
	if base == nil {
		return links
	}
	for _, l := range entry.Links {
		if len(l.Href) == 0 {
			continue
		}
		if strings.TrimPrefix(l.URL().Host, "www.") == strings.TrimPrefix(base.Host, "www.") {
			continue
		}
		if entry.InternalLink(base).Href == l.Href {
			continue
		}
		links = append(links, &l)
	}
	return links
}

// Link.URL()
func (link *Link) URL() (u *url.URL) {
	if link == nil || link.Href == "" {
		return u
	}
	u, _ = url.Parse(link.Href) // ignore error, return nil on failure
	return u
}

// FUNCTIONS
// =============================================================================

// Detected demonstrates regexp on []byte.
func Detected(doc []byte) bool {
	var patterns []*regexp.Regexp
	patterns = append(patterns, regexp.MustCompile(`<(feed|entry)\b[^>]*\bxmlns\s*=\s*('http://www\.w3\.org/2005/Atom'|"http://www\.w3\.org/2005/Atom")`))
	for _, pattern := range patterns {
		if pattern.Match(doc) {
			return true
		}
	}
	return false
}

func Parse(doc []byte) (*Atom, error) {
	var atom *Atom = &Atom{}
	err := xml.Unmarshal(doc, &atom)
	return atom, err
}

func First(values ...*Text) *Text {
	if len(values) == 0 {
		return nil
	}
	for _, v := range values {
		if v != nil && v.Body != "" {
			return v
		}
	}
	return values[0]
}

func HTML(text *Text) string {
	if text == nil {
		return ""
	}
	if text.Type == "html" || text.Type == "xhtml" {
		return text.Body
	}
	if !htmlTagPattern.MatchString(text.Body) && text.Body != "" {
		return "<p>" + text.Body + "</p>"
	}
	return text.Body
}
