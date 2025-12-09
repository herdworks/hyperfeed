package hyperfeed

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	hypergraph "github.com/herdworks/hypergraph-go/types"
)

func TestEntitySerialization(t *testing.T) {
	var websiteJson []byte
	var err error
	website := &Website{
		ID: "website-123",
		Entity: hypergraph.Entity{
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
