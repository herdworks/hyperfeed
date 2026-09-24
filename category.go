package hyperfeed

import "strings"

// Category identifies a feed category by its normalized tag and display label.
type Category struct {
	Tag   string `json:"tag,omitempty"`
	Label string `json:"label,omitempty"`
}

// NewCategory creates a category with its tag derived from label.
func NewCategory(label string) Category {
	return Category{Tag: NormalizeCategoryTag(label), Label: label}
}

// NormalizeCategoryTag derives a stable tag from a category label.
func NormalizeCategoryTag(label string) string {
	var result strings.Builder
	for _, character := range label {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' {
			result.WriteRune(character)
		}
	}
	return result.String()
}
