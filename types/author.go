package hyperfeed

import (
	"fmt"
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// ============================ Package Types =================================

type Author struct {
	ID          string     `json:"id,omitempty"` // stable HyperFeed Author identifier (e.g. hash of URL+name+email)
	Href        *href.Href `json:"href,omitempty"`
	Username    string     `json:"username,omitempty"`
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Email       string     `json:"email,omitempty"`
	Favicon     *href.Href `json:"avatar,omitempty"`
	Cover       *href.Href `json:"cover,omitempty"`
	Verified    bool       `json:"verified,omitempty"`
	Source      string     `json:"source,omitempty"` // source of author information (e.g. feed URL)
}

// ============================ Author Methods ================================

func (author *Author) IsEmpty() bool {
	if author == nil || author.Name == "" {
		return true
	}
	return false
}

func (author *Author) Identifier() string {
	if author == nil {
		return ""
	}
	if author.ID == "" {
		var id *url.URL = normalized.URL(author.Href.URL())
		var query url.Values = id.Query()
		if id == nil {
			id = &url.URL{}
		}
		query.Set("_username", author.Username)
		query.Set("_email", author.Email)
		query.Set("_verified", fmt.Sprintf("%v", author.Verified))
		query.Set("_source", author.Source)
		id.RawQuery = query.Encode()
		author.ID = normalized.Identifier(normalized.URL(id).String())
	}
	return author.ID
}

func (Author) DatabaseView() string {
	return "authors_view"
}
func (Author) DatabaseViewIdentifier() string {
	return "id"
}
func (Author) DatabaseObject() string {
	return "author"
}
func (Author) DatabaseTable() string {
	return "authors"
}
func (Author) DatabaseTableIdentifier() string {
	return "id"
}
func (Author) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"href",
		"username",
		"name",
		"description",
		"favicon",
		"cover",
		"verified",
		"email",
		"source",
	}
}
func (author *Author) DatabaseInsertValues() (values []any) {
	values = append(values, author.Identifier())
	values = append(values, author.Href.String())
	values = append(values, author.Username)
	values = append(values, author.Name)
	values = append(values, author.Description)
	values = append(values, author.Favicon.String())
	values = append(values, author.Cover.String())
	values = append(values, author.Verified)
	values = append(values, author.Email)
	values = append(values, author.Source)
	return values
}
func (author *Author) DatabaseUpdatesSet() []string {
	return []string{
		"[href] = excluded.href",
		"[username] = excluded.username",
		"[name] = excluded.name",
		"[description] = excluded.description",
		"[favicon] = excluded.favicon",
		"[cover] = excluded.cover",
		"[verified] = excluded.verified",
		"[email] = excluded.email",
		"[source] = excluded.source",
	}
}

func (author *Author) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%v|%s",
		author.Href.String(),
		author.Username,
		author.Name,
		author.Description,
		author.Favicon.String(),
		author.Cover.String(),
		author.Verified,
		author.Email,
	)
}
