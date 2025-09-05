package hyperfeed

import (
	"github.com/herdworks/hypertemplates-go/atom"
)

type Feed struct {
	Atom     *atom.Feed
	Metadata Metadata
}
type Feeds []*Feed
