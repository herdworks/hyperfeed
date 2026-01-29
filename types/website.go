package hyperfeed

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hypergraph "github.com/herdworks/hypergraph-go/types"
)

// TYPES
// =============================================================================
type Website struct {
	*DatabaseRecord
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
		if id, err = url.Parse(website.Entity.HTMLURL); err != nil {
			return normalized.Identifier("")
		}
		id = normalized.URL(id)
		website.ID = normalized.Identifier(id.String())
	}
	return website.ID
}

func (*Website) DatabaseObject() string {
	return "website"
}
func (*Website) DatabaseTable() string {
	return "websites"
}
func (*Website) DatabaseView() string {
	return "websites_view"
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
	values = append(values, sql.Named("id", website.Identifier()))
	values = append(values, sql.Named("html_url", website.HTMLURL))
	values = append(values, sql.Named("favicon_url", website.FaviconURL))
	values = append(values, sql.Named("cover_url", website.CoverURL))
	values = append(values, sql.Named("mentions_url", website.MentionsURL))
	values = append(values, sql.Named("email", website.Email))
	values = append(values, sql.Named("name", website.Name))
	values = append(values, sql.Named("username", website.Username))
	values = append(values, sql.Named("description", website.Description))
	values = append(values, sql.Named("verified", website.Verified))
	if website.Status == nil {
		values = append(values, sql.Named("status", "{}"))
	} else {
		statusBytes, _ := json.Marshal(website.Status)
		values = append(values, sql.Named("status", string(statusBytes)))
	}
	return values
}
func (website *Website) DatabaseUpdatesSet(_ []string) []string {
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
