package hyperfeed

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/herdworks/hq"
)

func TestEntitySerialization(t *testing.T) {
	var websiteJson []byte
	var err error
	website := &Website{
		ID: "website-123",
		Entity: hq.Entity{
			HTMLURL:     "https://calebhailey.com",
			FaviconURL:  "https://calebhailey.com/favicon.ico",
			CoverURL:    "https://calebhailey.com/cover.jpg",
			MentionsURL: "https://calebhailey.com/mentions",
			Email:       "contact@calebhailey.com",
			Name:        "Caleb Hailey",
			Username:    "@calebhailey.com",
			Description: "Founder @herd.works. RSS prosumer.",
			Verified:    true,
		},
		Status: &Status{
			UpdatedAt: time.Now(),
		},
	}
	if websiteJson, err = json.Marshal(website); err != nil {
		t.Fatalf("Failed to marshal website: %v", err)
	}
	fmt.Println(string(websiteJson))
}

func TestWebsiteFollowingURLSerializationAndDatabaseMapping(t *testing.T) {
	website := &Website{
		ID: "website-123",
		Entity: hq.Entity{
			FollowingURL: "https://example.com/following.opml",
		},
	}

	websiteJSON, err := json.Marshal(website)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded Website
	if err := json.Unmarshal(websiteJSON, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Entity.FollowingURL != website.Entity.FollowingURL {
		t.Fatalf("decoded following_url = %q; want %q", decoded.Entity.FollowingURL, website.Entity.FollowingURL)
	}

	if !containsString(website.DatabaseInsertColumns(), "following_url") {
		t.Fatalf("DatabaseInsertColumns() missing following_url")
	}
	foundValue := false
	for _, value := range website.DatabaseInsertValues() {
		arg, ok := value.(sql.NamedArg)
		if !ok || arg.Name != "following_url" {
			continue
		}
		foundValue = true
		if arg.Value != website.Entity.FollowingURL {
			t.Fatalf("following_url database value = %q; want %q", arg.Value, website.Entity.FollowingURL)
		}
	}
	if !foundValue {
		t.Fatalf("DatabaseInsertValues() missing following_url")
	}
	if !containsString(website.DatabaseUpdatesSet(website.DatabaseInsertColumns()), "[following_url] = [excluded].[following_url]") {
		t.Fatalf("DatabaseUpdatesSet() missing following_url update")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
