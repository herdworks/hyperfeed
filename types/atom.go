package hyperfeed

import (
	"encoding/xml"

	"github.com/herdworks/hypertemplates-go/atom"
)

type Atom struct {
	*atom.Feed
}

// RawFeed.Kind() compliance
func (a *Atom) Kind() FeedKind {
	return FeedKindAtom
}

// RawFeed.ItemCount() compliance
func (atom *Atom) ItemCount() int {
	return len(atom.Feed.Entry)
}

func ParseAtom(doc []byte) (atom *Atom, err error) {
	atom = &Atom{}
	err = atom.Unmarshal(doc)
	return atom, err
}

func (atom *Atom) Unmarshal(data []byte) error {
	return xml.Unmarshal(data, &atom.Feed)
}

func (atom *Atom) MarshalIndent(prefix string, indent string) ([]byte, error) {
	return xml.MarshalIndent(atom.Feed, prefix, indent)
}
