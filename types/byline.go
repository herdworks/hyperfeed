package hyperfeed

import (
	"fmt"
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// TYPES
// =============================================================================

type Byline struct {
	ID      string     `json:"id,omitempty"` // stable HyperFeed Byline identifier (e.g. hash of URL+name+email)
	Href    *href.Href `json:"href,omitempty"`
	Name    string     `json:"name,omitempty"`
	Email   string     `json:"email,omitempty"`
	Favicon *href.Href `json:"favicon,omitempty"`
}

// METHODS
// =============================================================================

func (byline *Byline) IsEmpty() bool {
	if byline == nil || byline.Name == "" {
		return true
	}
	return false
}

func (byline *Byline) Identifier() string {
	if byline == nil {
		return ""
	}
	if byline.ID == "" {
		var id *url.URL = normalized.URL(byline.Href.URL())
		var query url.Values = id.Query()
		if id == nil {
			id = &url.URL{}
		}
		query.Set("_name", byline.Name)
		query.Set("_email", byline.Email)
		id.RawQuery = query.Encode()
		byline.ID = normalized.Identifier(normalized.URL(id).String())
	}
	return byline.ID
}

func (*Byline) DatabaseView() string {
	return "bylines_view"
}
func (*Byline) DatabaseViewIdentifier() string {
	return "id"
}
func (*Byline) DatabaseObject() string {
	return "byline"
}
func (*Byline) DatabaseTable() string {
	return "bylines"
}
func (*Byline) DatabaseTableIdentifier() string {
	return "id"
}
func (*Byline) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"href",
		"name",
		"email",
	}
}
func (byline *Byline) DatabaseInsertValues() (values []any) {
	values = append(values, byline.Identifier())
	values = append(values, byline.Href.String())
	values = append(values, byline.Name)
	values = append(values, byline.Email)
	return values
}
func (byline *Byline) DatabaseUpdatesSet() []string {
	return []string{
		"[href] = excluded.href",
		"[name] = excluded.name",
		"[email] = excluded.email",
	}
}

func (byline *Byline) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s",
		byline.Href.String(),
		byline.Name,
		byline.Email,
	)
}
