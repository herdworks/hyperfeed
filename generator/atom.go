package generator

import (
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/atom"
)

// AtomGenerator serializes normalized HyperFeed values as Atom 1.0 XML.
type AtomGenerator struct{}

// Generate serializes feed as an Atom 1.0 document.
func (AtomGenerator) Generate(feed *hyperfeed.Feed) ([]byte, error) {
	if feed == nil {
		return nil, errors.New("atom generator requires a feed")
	}
	if feed.URL == nil || feed.URL.String() == "" {
		return nil, errors.New("atom generator requires feed URL")
	}
	result := &atom.Atom{
		ID:       feed.URL.String(),
		Title:    feed.Title,
		Subtitle: feed.Description,
		Updated:  atomTimestamp(feed.ModifiedAt),
	}
	links, err := atomLinks(feed.Links)
	if err != nil {
		return nil, err
	}
	result.Links = links
	result.Authors, result.Contributors = atomBylines(feed.Bylines)
	for _, item := range feed.Items {
		entry, err := atomEntry(item)
		if err != nil {
			return nil, err
		}
		result.Entries = append(result.Entries, entry)
	}
	return xml.MarshalIndent(result, "", "\t")
}

// Atom serializes a normalized feed as an Atom 1.0 document.
func Atom(feed *hyperfeed.Feed) ([]byte, error) {
	return AtomGenerator{}.Generate(feed)
}

func atomEntry(item *hyperfeed.Item) (*atom.Entry, error) {
	var links []atom.Link
	if item == nil {
		return nil, errors.New("atom generator cannot serialize a nil item")
	}
	authors, contributors := atomBylines(item.Bylines)
	links, err := atomEntryLinks(item)
	if err != nil {
		return nil, err
	}
	return &atom.Entry{
		ID:           item.GUID,
		Title:        item.Title,
		Authors:      authors,
		Contributors: contributors,
		Links:        links,
		Published:    atomTimestamp(item.PublishedAt),
		Updated:      atomTimestamp(item.ModifiedAt),
		Summary:      atom.Text{Type: "html", Body: item.Summary},
		Content:      atom.Text{Type: "html", Body: item.Content},
	}, nil
}

func atomBylines(bylines []hyperfeed.Byline) (authors []*atom.Author, contributors []*atom.Author) {
	for _, byline := range orderedBylines(bylines) {
		person := atomByline(byline)
		if strings.EqualFold(byline.Role, "contributor") {
			contributors = append(contributors, person)
			continue
		}
		authors = append(authors, person)
	}
	return authors, contributors
}

func atomByline(byline hyperfeed.Byline) *atom.Author {
	return &atom.Author{
		Name:    byline.Name,
		URI:     byline.Href.String(),
		Favicon: byline.Favicon.String(),
		Email:   byline.Email,
	}
}

func atomEntryLinks(item *hyperfeed.Item) ([]atom.Link, error) {
	links, err := atomLinks(item.Links)
	if err != nil {
		return nil, err
	}
	for _, attachment := range item.Attachments {
		if !attachment.Enclosure {
			continue
		}
		if attachment.Href == nil || attachment.Href.String() == "" {
			continue
		}
		if attachment.Size < 0 {
			return nil, fmt.Errorf("atom enclosure %q has negative size", attachment.Href.String())
		}
		link := atom.Link{
			Rel:   "enclosure",
			Href:  attachment.Href.String(),
			Type:  attachment.Mimetype,
			Title: attachment.Title,
		}
		if attachment.Size > 0 {
			if uint64(attachment.Size) > uint64(math.MaxUint) {
				return nil, fmt.Errorf("atom enclosure %q size exceeds platform uint", attachment.Href.String())
			}
			length := uint(attachment.Size)
			link.Length = &length
		}
		links = append(links, link)
	}
	return links, nil
}

func atomLinks(links []*hyperfeed.Link) ([]atom.Link, error) {
	result := make([]atom.Link, 0, len(links))
	for _, link := range links {
		if link == nil || link.Href == nil || link.Href.String() == "" {
			continue
		}
		atomLink, err := atomLink(*link)
		if err != nil {
			return nil, err
		}
		result = append(result, atomLink)
	}
	return result, nil
}

func atomLink(link hyperfeed.Link) (atom.Link, error) {
	result := atom.Link{
		Rel:      link.Rel,
		Href:     link.Href.String(),
		Type:     link.Mimetype,
		HrefLang: link.HrefLang,
		Title:    link.Title,
	}
	if link.Length == nil {
		return result, nil
	}
	if *link.Length < 0 {
		return atom.Link{}, fmt.Errorf("atom link %q has negative length", link.Href.String())
	}
	if uint64(*link.Length) > uint64(math.MaxUint) {
		return atom.Link{}, fmt.Errorf("atom link %q length exceeds platform uint", link.Href.String())
	}
	length := uint(*link.Length)
	result.Length = &length
	return result, nil
}

func atomTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return ""
	}
	return timestamp.Format(time.RFC3339)
}
