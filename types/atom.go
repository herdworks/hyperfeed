package hyperfeed

import (
	"encoding/xml"
	"fmt"
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

// RawFeed.GetURL() compliance
func (atom *Atom) GetURL() (baseurl *url.URL) {
	var link string
	var err error
	for _, l := range atom.Link {
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
func (atom *Atom) SetURL(u *url.URL) {
	if u == nil || u.String() == "" {
		return
	}
	for i, l := range atom.Link {
		if l.Rel == "self" {
			atom.Link[i].Href = u.String()
			return
		}
	}
}
// RawFeed.ItemCount() compliance
func (atom *Atom) ItemCount() int {
	return len(atom.Entry)
}

// RawFeed.ParseItems() compliance
func (atom *Atom) ParseItems(feedID string, added time.Time) (items []*Item, err error) {
	for _, atomentry := range atom.Entry {
		if item, err := atom.ParseItem(atomentry); err == nil {
			item.FeedID = feedID
			item.Added = added
			items = append(items, item)
		} else {
			return items, err
		}

	}
	return items, nil
}

func (atom *Atom) ParseFeedAuthors() (authors []Author, err error) {
	var author Author = Author{}
	var href string
	if atom.Author == nil {
		atom.Author = &AtomPerson{}
	}
	if atom.Author.Name != "" {
		author.Name = atom.Author.Name
	} else {
		author.Name = atom.Title
	}
	if atom.Author.Email != "" {
		author.Email = atom.Author.Email
	}
	if atom.Author.URI != "" {
		href = atom.Author.URI
	}
	if href == "" {
		for _, link := range atom.Link {
			if link.Rel == "alternate" || link.Rel == "" {
				href = link.Href
				break
			}
		}
	}
	if author.URL, err = url.Parse(href); err != nil {
		return nil, err
	}
	authors = append(authors, author)
	return authors, nil
}

func (atom *Atom) ParseItem(entry *AtomEntry) (item *Item, err error) {
	var published, updated time.Time
	if entry == nil {
		return nil, nil
	}
	item = &Item{}
	// item.ID = entry.ID
	item.URL = entry.AlternateURL()
	item.Title = entry.Title
	if published, err = time.Parse(time.RFC3339, entry.Published); err != nil {
		published = time.Time{}
	}
	if updated, err = time.Parse(time.RFC3339, entry.Updated); err != nil {
		updated = time.Time{}
	}
	item.Added = time.Now().UTC()
	item.Published = earliest(published, updated)
	item.Updated = latest(published, updated)
	if entry.Summary != nil {
		item.Summary = entry.Summary.Body
	}
	if entry.Content != nil {
		item.Content = entry.Content.Body
	}
	if item.Authors, err = atom.ParseAuthors(entry); err != nil {
		return item, err
	}
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

func (atom *Atom) ParseAuthors(entry *AtomEntry) (authors []Author, err error) {
	var authorURL *url.URL
	if authorURL, err = atom.EntryAuthorURL(entry); err != nil {
		return nil, err
	}
	if entry.Author == nil {
		authors = append(authors, Author{
			Name: first(atom.Author.Name, atom.Title),
			Email: atom.Author.Email,
			URL:   atom.GetURL(),
		})
	} else {
		authors = append(authors, Author{
			Name:  entry.Author.Name,
			Email: entry.Author.Email,
			URL:   authorURL,
		})
	}
	for _, contributor := range entry.Contributor {
		var contributorURL *url.URL
		if contributorURL, err = url.Parse(contributor.URI); err != nil {
			fmt.Printf("warning parsing contributor URI: %v\n", err)
			contributorURL = &url.URL{}
		}
		authors = append(authors, Author{
			Name:  contributor.Name,
			Email: contributor.Email,
			URL:   contributorURL,
		})
	}
	return authors, nil
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

func (atom *Atom) EntryAuthorURL(entry *AtomEntry) (u *url.URL, err error) {
	if entry.Author != nil && entry.Author.URI != "" {
		return url.Parse(entry.Author.URI)
	}
	for _, link := range atom.Link {
		if link.Rel == "alternate" || link.Rel == "" {
			return url.Parse(link.Href) // use feed link as default author URL
		}
	}
	return &url.URL{}, nil
}

func (atom *Atom) Unmarshal(data []byte) error {
	return xml.Unmarshal(data, &atom)
}

func (atom *Atom) MarshalIndent(prefix string, indent string) ([]byte, error) {
	return xml.MarshalIndent(atom, prefix, indent)
}

// ============================ AtomEntry Methods =============================
func (entry *AtomEntry) AlternateURL() *url.URL {
	for _, link := range entry.Link {
		if link.Rel == "alternate" || link.Rel == "" {
			if u, err := url.Parse(link.Href); err == nil {
				return u
			}
		}
	}
	return &url.URL{}
}

// ============================ Helper Functions ==============================

func ParseAtom(doc []byte) (atom *Atom, err error) {
	atom = &Atom{}
	err = atom.Unmarshal(doc)
	return atom, err
}

