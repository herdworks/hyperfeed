package hyperfeed

import (
	"encoding/json"
	"fmt"
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hypergraph "github.com/herdworks/hypergraph-go/types"
)

// TYPES
// =============================================================================
type Website struct {
	hypergraph.Entity

	ID     string  `json:"id"`
	Status *Status `json:"status"`
}

// METHODS
// =============================================================================

func (website *Website) Identifier() string {
	var id *url.URL
	var err error
	if website == nil {
		return normalized.Identifier("") // the unknown entity
	}
	if website.ID == "" {
		if id, err = url.Parse(website.HTMLURL); err != nil {
			return normalized.Identifier("")
		}
		id = normalized.URL(id)
		website.ID = normalized.Identifier(id.String())
	}
	return website.ID
}

func (*Website) DatabaseView() string {
	return "websites_view"
}
func (*Website) DatabaseViewIdentifier() string {
	return "id"
}
func (*Website) DatabaseObject() string {
	return "website"
}
func (*Website) DatabaseTable() string {
	return "websites"
}
func (*Website) DatabaseTableIdentifier() string {
	return "id"
}
func (*Website) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"html_url",
		"favicon_url",
		"cover_url",
		"mentions_url",
		"email",
		"name",
		"username",
		"description",
		"verified",
		"status",
	}
}
func (website *Website) DatabaseInsertValues() (values []any) {
	values = append(values, website.Identifier())
	values = append(values, website.HTMLURL)
	values = append(values, website.FaviconURL)
	values = append(values, website.CoverURL)
	values = append(values, website.MentionsURL)
	values = append(values, website.Email)
	values = append(values, website.Name)
	values = append(values, website.Username)
	values = append(values, website.Description)
	values = append(values, website.Verified)
	if website.Status == nil {
		values = append(values, "{}")
	} else {
		statusBytes, _ := json.Marshal(website.Status)
		values = append(values, string(statusBytes))
	}
	return values
}
func (website *Website) DatabaseUpdatesSet() []string {
	return []string{
		"[html_url] = [excluded].[html_url]",
		"[favicon_url] = [excluded].[favicon_url]",
		"[cover_url] = [excluded].[cover_url]",
		"[mentions_url] = [excluded].[mentions_url]",
		"[email] = [excluded].[email]",
		"[name] = [excluded].[name]",
		"[username] = [excluded].[username]",
		"[description] = [excluded].[description]",
		"[verified] = [excluded].[verified]",
	}
}

func (website *Website) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%v|%s",
		website.HTMLURL,
		website.FaviconURL,
		website.CoverURL,
		website.Username,
		website.Name,
		website.Description,
		website.Verified,
		website.Email,
	)
}
