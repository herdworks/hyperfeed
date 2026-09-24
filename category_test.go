package hyperfeed

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewCategory(t *testing.T) {
	category := NewCategory("Open Source & R&D_2026!")
	if category.Label != "Open Source & R&D_2026!" {
		t.Fatalf("label = %q", category.Label)
	}
	if category.Tag != "OpenSourceRD_2026" {
		t.Fatalf("tag = %q, want %q", category.Tag, "OpenSourceRD_2026")
	}
}

func TestCategoryJSONIsObjectOnly(t *testing.T) {
	item := Item{Categories: []Category{NewCategory("Open Source")}}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"categories":[{"tag":"OpenSource","label":"Open Source"}]`) {
		t.Fatalf("JSON = %s", data)
	}
	var decoded Item
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Categories) != 1 || decoded.Categories[0] != item.Categories[0] {
		t.Fatalf("decoded categories = %#v", decoded.Categories)
	}
	if err := json.Unmarshal([]byte(`{"categories":["Open Source"]}`), &decoded); err == nil {
		t.Fatal("legacy string category decoded successfully")
	}
}

func TestCategoryJSONOmitsUnspecifiedFields(t *testing.T) {
	data, err := json.Marshal(Category{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("JSON = %s, want {}", data)
	}
}
