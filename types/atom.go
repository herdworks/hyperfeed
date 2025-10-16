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
	Links   []AtomLink    `xml:"link"`
	Authors []*AtomAuthor `xml:"author,omitempty"`
	Entries []*AtomEntry  `xml:"entry"`
	Updated AtomTimeStamp `xml:"updated"`
}

func (atom *Atom) Render() (string, error) {
	atomBytes, err := xml.MarshalIndent(atom, "", "\t")
	if err != nil {
		return "", err
	}
	return string(atomBytes), nil
}

type AtomEntry struct {
	Title        string        `xml:"title"`
	ID           string        `xml:"id"`
	Link         []AtomLink    `xml:"link"`
	Published    AtomTimeStamp `xml:"published"`
	Updated      AtomTimeStamp `xml:"updated"`
	Authors      []*AtomAuthor `xml:"author"`
	Contributors []*AtomAuthor `xml:"contributor"` // extension from golang.org/x/tools/blog/atom
	Summary      *AtomText     `xml:"summary"`
	Content      *AtomText     `xml:"content"`
}

type AtomLink struct {
	Rel      string `xml:"rel,attr,omitempty"`
	Href     string `xml:"href,attr"`
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   uint   `xml:"length,attr,omitempty"`
}

type AtomAuthor struct {
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

// RawFeed.GetURL() compliance
func (atom *Atom) GetURL() (baseurl *url.URL) {
	var link string
	var err error
	for _, l := range atom.Links {
		if l.Href == "" {
			continue
		}
		if l.Rel == "alternate" || l.Rel == "" {
			link = l.Href // use feed link as default author URL
			break
		}
	}
	if baseurl, err = url.Parse(link); err != nil {
		return nil
	}
	return baseurl
}

// RawFeed.SetURL() compliance
func (atom *Atom) SetURL(u *url.URL) {
	if u == nil || u.String() == "" {
		return
	}
	atom.Links = append([]AtomLink{{Href: u.String(), Rel: ""}}, atom.Links...)
}

// RawFeed.ItemCount() compliance
func (atom *Atom) ItemCount() int {
	return len(atom.Entries)
}

// RawFeed.ParseFeedAuthors() compliance
func (atom *Atom) ParseFeedAuthors() (authors []Author) {
	for _, aa := range atom.Authors {
		var author Author = Author{}
		if aa == nil {
			aa = &AtomAuthor{}
		}
		if aa.Name != "" {
			author.Name = aa.Name
		} else {
			author.Name = atom.Title
		}
		if aa.Email != "" {
			author.Email = aa.Email
		}
		if aa.GetURL() != nil {
			author.URL = aa.GetURL()
		} else {
			author.URL = atom.GetURL()
		}
		authors = append(authors, author)
	}
	return authors
}

// RawFeed.ParseItems() compliance
func (atom *Atom) ParseItems(feedID string, added time.Time) (items []*Item, err error) {
	for _, atomentry := range atom.Entries {
		if item, err := atom.ParseItem(atomentry); err == nil {
			item.FeedID = feedID
			item.Added = added
			item.Updated = added
			items = append(items, item)
		} else {
			return items, err
		}

	}
	return items, nil
}

func (atom *Atom) ParseItem(entry *AtomEntry) (item *Item, err error) {
	var published, modified time.Time
	if entry == nil {
		return nil, nil
	}
	item = &Item{}
	if item.URL = entry.GetURL(); !item.URL.IsAbs() && atom.GetURL() != nil {
		item.URL = atom.GetURL().ResolveReference(item.URL) // resolve relative URLs against the feed URL
	}
	item.Title = entry.Title
	if published, err = time.Parse(time.RFC3339, entry.Published); err != nil {
		published = time.Time{}
	}
	if modified, err = time.Parse(time.RFC3339, entry.Updated); err != nil {
		modified = time.Time{}
	}
	item.Added = time.Now().UTC()
	item.Published = earliest(published, modified)
	item.Modified = latest(published, modified)
	if entry.Summary != nil {
		item.Summary = entry.Summary.Body
	}
	if entry.Content != nil {
		item.Content = entry.Content.Body
	}
	item.Authors = atom.ParseEntryAuthors(entry)
	if item.Attachments, err = atom.ParseAttachments(entry); err != nil {
		return item, err
	}
	return item, nil
}

func (atom *Atom) ParseItemURLs(item *Item, entry *AtomEntry) {
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
			if a, err := atom.ParseAttachment(entry, &link); err == nil && a != nil {
				item.Attachments = append(item.Attachments, *a)
			}
		default:
			item.Links = append(item.Links, u)
		}
	}
}

func (atom *Atom) ParseEntryAuthors(entry *AtomEntry) (authors []Author) {
	if len(entry.Authors) == 0 && len(entry.Contributors) == 0 {
		return atom.ParseFeedAuthors()
	} else {
		// loop over authors and contributors
		for _, a := range append(entry.Authors, entry.Contributors...) {
			var author Author = Author{Name: a.Name, Email: a.Email}
			if author.URL = a.GetURL(); author.URL == nil {
				author.URL = atom.GetURL()
			}
			authors = append(authors, author)
		}
	}
	return authors
}

func (atom *Atom) ParseAttachments(entry *AtomEntry) (attachments []Attachment, err error) {
	for _, link := range entry.Link {
		if attachment, err := atom.ParseAttachment(entry, &link); err == nil && attachment != nil {
			attachments = append(attachments, *attachment)
		} else if err != nil {
			return attachments, err
		}
	}
	return attachments, nil
}
func (atom *Atom) ParseAttachment(entry *AtomEntry, link *AtomLink) (attachment *Attachment, err error) {
	attachment = &Attachment{}
	if link == nil || link.Rel != "enclosure" {
		return nil, nil // noop
	}
	if u, err := url.Parse(link.Href); err == nil && u != nil {
		attachment.URL = u
	}
	attachment.Title = link.Title
	attachment.Size = int64(link.Length)
	attachment.Mimetype = link.Type
	for _, l := range entry.Link {
		if l.Rel != "alternate" {
			continue
		}
		attachment.ItemID = entry.ID
	}
	return attachment, nil
}

func (atom *Atom) Unmarshal(data []byte) error {
	return xml.Unmarshal(data, &atom)
}

func (atom *Atom) MarshalIndent(prefix string, indent string) ([]byte, error) {
	return xml.MarshalIndent(atom, prefix, indent)
}

// ============================ AtomEntry Methods =============================

// RawItem.GetURL() compliance
func (entry *AtomEntry) GetURL() *url.URL {
	for _, link := range entry.Link {
		if link.Rel == "alternate" || link.Rel == "" {
			if u, err := url.Parse(link.Href); err == nil {
				return u
			}
		}
	}
	return &url.URL{}
}

// ============================ AtomLink Methods ==============================

func (link *AtomLink) URL() (u *url.URL) {
	if link == nil || link.Href == "" {
		return u
	}
	u, _ = url.Parse(link.Href) // ignore error, return nil on failure
	return u
}

// ============================ AtomAuthor Methods ============================

func (author *AtomAuthor) GetURL() *url.URL {
	if author == nil || author.URI == "" {
		return nil
	}
	u, _ := url.Parse(author.URI)
	return u
}

// ============================ Helper Functions ==============================

func ParseAtom(doc []byte) (atom *Atom, err error) {
	atom = &Atom{}
	err = atom.Unmarshal(doc)
	return atom, err
}
