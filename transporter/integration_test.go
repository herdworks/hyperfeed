//go:build integration

package transporter

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ============================================================================
// Integration Tests
//
// These tests make real network requests and are not run by default.
// To run these tests, use:
//
//   go test ./... -tags=integration
// ============================================================================

func TestTransporterFetchIntegration(t *testing.T) {
	tests := []struct {
		name    string
		request *Request
		err     error
	}{
		{
			"feeds.allenpike.com/feed/",
			&Request{URL: &url.URL{Scheme: "https", Host: "feeds.allenpike.com", Path: "/feed/"}},
			nil,
		},
		{
			"www.thisiscolossal.com/feed/",
			&Request{URL: &url.URL{Scheme: "https", Host: "www.thisiscolossal.com", Path: "/feed/"}},
			nil,
		},
		{
			"furbo.org/feed/json",
			&Request{URL: &url.URL{Scheme: "https", Host: "furbo.org", Path: "/feed/json"}},
			nil,
		},
		{
			"daringfireball.net/feeds/json",
			&Request{URL: &url.URL{Scheme: "https", Host: "daringfireball.net", Path: "/feeds/json"}},
			nil,
		},
		{
			"inessential.com/feed.json",
			&Request{URL: &url.URL{Scheme: "https", Host: "inessential.com", Path: "/feed.json"}},
			nil,
		},
		{
			"feeds.kottke.org/json",
			&Request{URL: &url.URL{Scheme: "https", Host: "feeds.kottke.org", Path: "/json"}},
			nil,
		},
		{
			"jvns.ca/atom.xml",
			&Request{URL: &url.URL{Scheme: "https", Host: "jvns.ca", Path: "/atom.xml"}},
			nil,
		},
		{
			"www.manton.org/feed/json",
			&Request{URL: &url.URL{Scheme: "https", Host: "www.manton.org", Path: "/feed/json"}},
			nil,
		},
		{
			"vincode.io/feed.xml",
			&Request{URL: &url.URL{Scheme: "https", Host: "vincode.io", Path: "/feed.xml"}},
			nil,
		},
		{
			"mjtsai.com/blog/feed/",
			&Request{URL: &url.URL{Scheme: "https", Host: "mjtsai.com", Path: "/blog/feed/"}},
			nil,
		},
		{
			"netnewswire.blog/feed.json",
			&Request{URL: &url.URL{Scheme: "https", Host: "netnewswire.blog", Path: "/feed.json"}},
			nil,
		},
		{
			"onefoottsunami.com/feed/json/",
			&Request{URL: &url.URL{Scheme: "https", Host: "onefoottsunami.com", Path: "/feed/json/"}},
			nil,
		},
		{
			"scripting.com/rss.xml",
			&Request{URL: &url.URL{Scheme: "http", Host: "scripting.com", Path: "/rss.xml"}},
			nil,
		},
		{
			"feedpress.me/sixcolors?type=xml",
			&Request{URL: &url.URL{Scheme: "https", Host: "feedpress.me", Path: "/sixcolors", RawQuery: "type=xml"}},
			nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var transporter *Transporter = &Transporter{}
			var response *Response
			var err error
			if response, err = transporter.Fetch(context.TODO(), tc.request); tc.err != nil && err == nil {
				t.Fatalf("expected error %v, got %v", tc.err, err)
			} else if tc.err != nil && errors.Is(err, tc.err) {
				t.Logf("got expected error\n")
				return
			}
			t.Logf("%s %+v", tc.request.Request.Method, tc.request.URL.String())
			t.Logf("%+v %s", response.Response.StatusCode, http.StatusText(response.Response.StatusCode))
			for k, v := range response.Response.Header {
				if isVendoredResponseHeader(k) || isDeprecatedResponseHeader(k) || isExperimentalResponseHeader(k) {
					continue // skip vendored-, deprecated-, and experimental- headers
				}
				values := strings.Join(v, ", ")
				t.Logf("%s: %+v", http.CanonicalHeaderKey(k), values)
			}
			t.Logf("response body contains %d bytes", len(response.Bytes))

			var feed *hyperfeed.Feed = &hyperfeed.Feed{URL: tc.request.URL}
			kind := feed.DetectKind(response.Bytes)
			t.Logf("detected feed kind: %s", kind)
		})
	}
}
