package hyperfeed

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/herdworks/hyperfeed/href"
)

func TestBylinePublishingFieldsJSON(t *testing.T) {
	assertJSONOmitted(t, Byline{}, "role", "order")
	byline := Byline{Name: "Editor", Role: "contributor", Order: 2}
	data, err := json.Marshal(byline)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Byline
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(byline, decoded) {
		t.Fatalf("round trip changed supplied values:\n%s\n%#v", data, decoded)
	}
}

func TestBylinePublishingFieldsDoNotChangeIdentityOrDatabaseValues(t *testing.T) {
	u, err := href.Parse("https://example.com/person")
	if err != nil {
		t.Fatal(err)
	}
	byline := Byline{Href: u, Name: "Person", Email: "person@example.com"}
	bylineID, bylineHash, bylineValues := byline.Identifier(), byline.Hash(), byline.DatabaseInsertValues()
	byline.Role, byline.Order = "contributor", 12
	if byline.Identifier() != bylineID || byline.Hash() != bylineHash || !reflect.DeepEqual(bylineValues, byline.DatabaseInsertValues()) {
		t.Fatal("attribution metadata changed shared-person identity or persistence")
	}
}
