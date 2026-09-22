// Package generator serializes normalized HyperFeed domain values.
package generator

import (
	"cmp"
	"slices"

	"github.com/herdworks/hyperfeed"
)

// Generator serializes a normalized feed into a format-specific document.
type Generator interface {
	Generate(*hyperfeed.Feed) ([]byte, error)
}

func orderedBylines(bylines []hyperfeed.Byline) []hyperfeed.Byline {
	ordered := append([]hyperfeed.Byline(nil), bylines...)
	slices.SortStableFunc(ordered, func(left hyperfeed.Byline, right hyperfeed.Byline) int {
		switch {
		case left.Order == 0 && right.Order == 0:
			return 0
		case left.Order == 0:
			return 1
		case right.Order == 0:
			return -1
		default:
			return cmp.Compare(left.Order, right.Order)
		}
	})
	return ordered
}
