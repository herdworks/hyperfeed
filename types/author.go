package hyperfeed

import (
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
)

// ============================ Package Types =================================

type Author struct {
	ID       string   `json:"id,omitempty"` // stable HyperFeed Author identifier (e.g. hash of URL+name+email)
	URL      *url.URL `json:"url,omitempty"`
	Username string   `json:"username,omitempty"`
	Name     string   `json:"name,omitempty"`
	Email    string   `json:"email,omitempty"`
	Favicon  *url.URL `json:"avatar,omitempty"`

	Href string `json:"-"` // string representation of Author.URL
}

func (author *Author) Identifier() any {
	if author != nil && author.ID != "" {
		return author.ID
	}
	var id *url.URL = normalized.URL(author.URL)
	var query url.Values = id.Query()
	if id == nil {
		id = &url.URL{}
	}
	query.Set("_name", author.Name)
	query.Set("_email", author.Email)
	id.RawQuery = query.Encode()
	return normalized.Identifier(normalized.URL(id).String())
}

func (author *Author) RowTable() string {
	return "authors"
}
func (author *Author) RowIdentifier() string {
	return "id"
}
func (author *Author) RowColumns() []string {
	return []string{
		"id",
		"href",
		"name",
		"username",
		"favicon",
		"email",
	}
}
func (author *Author) RowArguments() (args []any) {
	args = append(args, author.Identifier())
	if author.URL != nil {
		args = append(args, author.URL.String())
	} else {
		args = append(args, "")
	}
	args = append(args, author.Name)
	args = append(args, author.Username)
	if author.Favicon != nil {
		args = append(args, author.Favicon.String())
	} else {
		args = append(args, "")
	}
	args = append(args, author.Email)
	return args
}
func (author *Author) RowUpdates() [][2]string {
	return [][2]string{
		{"href", "excluded.href"},
		{"name", "excluded.name"},
		{"username", "excluded.username"},
		{"favicon", "excluded.favicon"},
		{"email", "excluded.email"},
	}
}

func (author *Author) RowTargets() (targets []any) {
	targets = append(targets, &author.ID)
	targets = append(targets, &author.Href)
	targets = append(targets, &author.Name)
	targets = append(targets, &author.Username)
	targets = append(targets, &author.Favicon)
	targets = append(targets, &author.Email)
	return targets
}
