package hyperfeed

import (
	"encoding/xml"
	"net/url"
	"regexp"
	"time"
)

// Type atom defines XML data structures for an Atom feed.
//
// reference: https://www.ietf.org/rfc/rfc4287.txt

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

type Atom struct {
	XMLName xml.Name      `xml:"http://www.w3.org/2005/Atom feed"`
	XMLNS   string        `xml:"xmlns:hypergraph,attr"`
	Title   string        `xml:"title"`
	ID      string        `xml:"id"`
	Link    []AtomLink    `xml:"link"`
	Updated AtomTimeStamp `xml:"updated"`
	Author  *AtomPerson   `xml:"author,omitempty"`
	Entry   []*AtomEntry  `xml:"entry"` // TODO: should this be renamed to "Entries"?
}

func (atom *Atom) Render() (string, error) {
	atomBytes, err := xml.MarshalIndent(atom, "", "\t")
	if err != nil {
		return "", err
	}
	return string(atomBytes), nil
}

type AtomEntry struct {
	Title       string        `xml:"title"`
	ID          string        `xml:"id"`
	Link        []AtomLink    `xml:"link"`
	Published   AtomTimeStamp `xml:"published"`
	Updated     AtomTimeStamp `xml:"updated"`
	Author      *AtomPerson   `xml:"author"`
	Contributor []*AtomPerson `xml:"contributor"` // extension from golang.org/x/tools/blog/atom
	Summary     *AtomText     `xml:"summary"`
	Content     *AtomText     `xml:"content"`
}

type AtomLink struct {
	Rel      string `xml:"rel,attr,omitempty"`
	Href     string `xml:"href,attr"`
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   uint   `xml:"length,attr,omitempty"`
}

type AtomPerson struct {
	Name     string `xml:"name"`
	URI      string `xml:"uri,omitempty"`
	Username string `xml:"hypergraph:username,omitempty"`
	Favicon  string `xml:"hypergraph:favicon,omitempty"`
	Email    string `xml:"email,omitempty"`
	InnerXML string `xml:",innerxml"`
}

type AtomText struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type AtomTimeStamp = string

// AtomDetected demonstrates regexp on []byte.
func AtomDetected(doc []byte) bool {
	var patterns []*regexp.Regexp
	patterns = append(patterns, regexp.MustCompile(`<(feed|entry)\b[^>]*\bxmlns\s*=\s*('http://www\.w3\.org/2005/Atom'|"http://www\.w3\.org/2005/Atom")`))
	for _, pattern := range patterns {
		if pattern.Match(doc) {
			return true
		}
	}
	return false
}

// ============================ Atom Methods ==================================

// RawFeed.Kind() compliance
func (a *Atom) Kind() FeedKind {
	return FeedKindAtom
}

// RawFeed.ItemCount() compliance
func (atom *Atom) ItemCount() int {
	return len(atom.Entry)
}

// RawFeed.ParseItems() compliance
func (atom *Atom) ParseItems() (items []*Item) {
	for _, atomentry := range atom.Entry {
		item, err := atom.ParseItem(atomentry)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (atom *Atom) ParseItem(entry *AtomEntry) (item *Item, err error) {
	if entry == nil {
		return nil, nil
	}

	item = &Item{}
	item.Added = time.Now().UTC()
	item.ID = entry.ID
	item.Title = entry.Title
	if t, err := time.Parse(time.RFC3339, entry.Published); err == nil {
		item.Published = t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, entry.Updated); err == nil {
		item.Updated = t.UTC()
	}
	if entry.Summary != nil {
		item.Summary = entry.Summary.Body
	}
	if entry.Content != nil {
		item.Content = entry.Content.Body
	}
	for _, link := range entry.Link {
		var u *url.URL
		var err error
		if u, err = url.Parse(link.Href); err != nil {
			continue
		}
		switch link.Rel {
		case "alternate", "":
			item.URL = u
		case "enclosure":
			item.Attachments = append(item.Attachments, Attachment{
				URL: u,
			})
		default:
			item.Links = append(item.Links, u)
		}
	}

	if entry.Author != nil {
		item.Authors = append(item.Authors, Author{
			Name:  entry.Author.Name,
			Email: entry.Author.Email,
		})
	}
	for _, contributor := range entry.Contributor {
		item.Authors = append(item.Authors, Author{
			Name:  contributor.Name,
			Email: contributor.Email,
		})
	}
	return item, nil
}

func (atom *Atom) Unmarshal(data []byte) error {
	return xml.Unmarshal(data, &atom)
}

func (atom *Atom) MarshalIndent(prefix string, indent string) ([]byte, error) {
	return xml.MarshalIndent(atom, prefix, indent)
}

// ============================ Helper Functions ==============================

func ParseAtom(doc []byte) (atom *Atom, err error) {
	atom = &Atom{}
	err = atom.Unmarshal(doc)
	return atom, err
}
