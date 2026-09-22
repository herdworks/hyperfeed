package hyperfeed

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/herdworks/hyperfeed/href"
)

func TestLinkPublishingFieldsJSON(t *testing.T) {
	assertJSONOmitted(t, Link{}, "rel", "mimetype", "href_lang", "length", "origin")
	zero := int64(0)
	link := Link{Rel: "alternate", Mimetype: "text/html", HrefLang: "en", Length: &zero, Origin: "metadata"}
	data, err := json.Marshal(link)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Link
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(link, decoded) {
		t.Fatalf("round trip changed supplied values:\n%s\n%#v", data, decoded)
	}
}

func TestLinkPublishingFieldsDoNotChangeIdentityOrDatabaseValues(t *testing.T) {
	u, err := href.Parse("https://example.com/person")
	if err != nil {
		t.Fatal(err)
	}
	link := Link{Href: u, Title: "Person", Weight: 0.75}
	linkID, linkHash, linkValues := link.Identifier(), link.Hash(), link.DatabaseInsertValues()
	length := int64(42)
	link.Rel, link.Mimetype, link.HrefLang, link.Length, link.Origin = "alternate", "text/html", "en", &length, "metadata"
	if link.Identifier() != linkID || link.Hash() != linkHash || !reflect.DeepEqual(linkValues, link.DatabaseInsertValues()) {
		t.Fatal("syndication attributes changed link identity or persistence")
	}
}

// TestParseLinksSkipsUnparseableLink reproduces the Daring Fireball "The Talk
// Show" failure: a malformed <a href="..."> in an item's show notes (an
// unterminated quote made the HTML tokenizer capture a value containing a
// newline) caused ParseLinks to leak the raw url.Parse error out of its named
// return, which aborted the entire feed refresh. A single unparseable link must
// be skipped, not fatal.
func TestParseLinksSkipsUnparseableLink(t *testing.T) {
	itemURL, err := href.Parse("https://daringfireball.net/thetalkshow/ep")
	if err != nil {
		t.Fatalf("href.Parse() error = %v", err)
	}
	item := &Item{URL: itemURL}

	// The second anchor's href contains a newline (an invalid control
	// character), so url.Parse rejects it. It is positioned last so the
	// pre-fix named-return leak would surface it as a returned error.
	body := "<ul>" +
		`<li><a href="https://example.com/page">good</a></li>` +
		"<li><a href=\"https://example.com/a\nb\">bad</a></li>" +
		"</ul>"

	links, err := ParseLinks(item, []byte(body), io.Discard)
	if err != nil {
		t.Fatalf("ParseLinks() error = %v, want nil (bad link should be skipped)", err)
	}

	var foundGood bool
	for _, l := range links {
		if l == nil || l.Href == nil {
			continue
		}
		got := l.Href.String()
		if strings.ContainsAny(got, "\n\r") {
			t.Fatalf("ParseLinks() returned a link with a control character: %q", got)
		}
		if got == "https://example.com/page" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Fatalf("ParseLinks() dropped the valid link; got links: %s", linkURLs(links))
	}
}

func linkURLs(links []*Link) string {
	var urls []string
	for _, l := range links {
		if l != nil && l.Href != nil {
			urls = append(urls, l.Href.String())
		}
	}
	return strings.Join(urls, ", ")
}
