package hyperfeed

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	normalized "github.com/herdworks/hyperfeed/internal/normalization"
)

// TYPES
// =============================================================================

type Activity struct {
	*DatabaseRecord

	ID        string         `json:"id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
	Kind      ActivityKind   `json:"kind"`
	URI       *url.URL       `json:"uri,omitempty"` // url scheme
	Seen      bool           `json:"bool"`
	Data      map[string]any `json:"data"`
}

type ActivityKind string

const (
	DiscoveryActivity ActivityKind = "discovery"
)

// METHODS
// =============================================================================

func (activity *Activity) Identifier() string {
	if activity == nil {
		return ""
	}
	if activity.ID == "" {
		var id *url.URL = normalized.URL(activity.URI)
		var query url.Values = id.Query()
		if id == nil {
			id = &url.URL{}
		}
		query.Set("kind", activity.Kind.String())
		query.Set("created", activity.CreatedAt.Format(time.RFC3339))
		id.RawQuery = query.Encode()
		activity.ID = normalized.Identifier(normalized.URL(id).String())
	}
	return activity.ID
}

func (*Activity) DatabaseObject() string {
	return "activity"
}
func (*Activity) DatabaseTable() string {
	return "activities"
}
func (*Activity) DatabaseView() string {
	return "activities_view"
}
func (*Activity) DatabaseInsertColumns() []string {
	return []string{
		"id",
		"created_at",
		"updated_at",
		"kind",
		"uri",
		"seen",
		"data",
	}
}
func (activity *Activity) DatabaseInsertValues() (values []any) {
	values = append(values, sql.Named("id", activity.Identifier()))
	values = append(values, sql.Named("created_at", activity.CreatedAt.Format(time.RFC3339)))
	values = append(values, sql.Named("updated_at", activity.UpdatedAt.Format(time.RFC3339)))
	values = append(values, sql.Named("kind", activity.Kind.String()))
	if activity.URI != nil {
		values = append(values, sql.Named("uri", activity.URI.String()))
	} else {
		values = append(values, sql.Named("uri", nil))
	}
	if dataBytes, _ := json.Marshal(activity.Data); dataBytes != nil {
		values = append(values, sql.Named("data", string(dataBytes)))
	} else {
		values = append(values, sql.Named("data", "{}"))
	}
	return values
}

func (activity *Activity) Hash() string {
	return fmt.Sprintf(
		"%s|%s|%s",
		activity.Identifier(),
		activity.Kind.String(),
		activity.URI.String(),
	)
}

func (kind ActivityKind) String() string {
	return string(kind)
}
