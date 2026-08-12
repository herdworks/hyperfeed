package hyperfeed

import (
	"database/sql"
	"fmt"
	"net/url"

	"github.com/herdworks/hyperfeed/href"
	normalized "github.com/herdworks/hyperfeed/internal/normalization"
)

// TYPES
// =============================================================================

type Byline struct {
	*DatabaseRecord

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

func (*Byline) DatabaseObject() string {
	return "byline"
}
func (*Byline) DatabaseTable() string {
	return "bylines"
}
func (*Byline) DatabaseView() string {
	return "bylines_view"
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
	values = append(values, sql.Named("id", byline.Identifier()))
	values = append(values, sql.Named("href", byline.Href.String()))
	values = append(values, sql.Named("name", byline.Name))
	values = append(values, sql.Named("email", byline.Email))
	return values
}

func (byline *Byline) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s",
		byline.Href.String(),
		byline.Name,
		byline.Email,
	)
}
