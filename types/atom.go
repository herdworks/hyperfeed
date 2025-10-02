package hyperfeed

import (
	"encoding/xml"
	"net/url"
	"time"

	"github.com/herdworks/hypertemplates-go/atom"
)

type Atom struct {
	*atom.Feed
}

// ============================ Atom Methods ==================================

// RawFeed.Kind() compliance
func (a *Atom) Kind() FeedKind {
	return FeedKindAtom
}

// RawFeed.ItemCount() compliance
func (atom *Atom) ItemCount() int {
	return len(atom.Feed.Entry)
}

// RawFeed.ParseItems() compliance
func (atom *Atom) ParseItems() (items []*Item) {
	for _, atomentry := range atom.Feed.Entry {
		item, err := atom.ParseItem(atomentry)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (atom *Atom) ParseItem(entry *atom.Entry) (item *Item, err error) {
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
	return xml.Unmarshal(data, &atom.Feed)
}

func (atom *Atom) MarshalIndent(prefix string, indent string) ([]byte, error) {
	return xml.MarshalIndent(atom.Feed, prefix, indent)
}

// ============================ Helper Functions ==============================

func ParseAtom(doc []byte) (atom *Atom, err error) {
	atom = &Atom{}
	err = atom.Unmarshal(doc)
	return atom, err
}
