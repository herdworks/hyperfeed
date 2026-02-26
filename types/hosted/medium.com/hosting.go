package medium

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// medium.URL() converts subdomained urls like https://username.medium.com/article to https://medium.com/@username/article
func URL(in *url.URL) (out *url.URL) {
	var username string
	if in == nil || strings.HasPrefix(in.Path, "/@") {
		return in
	}

	// parse subdomain of medium.com as username
	if components := strings.Split(in.Hostname(), "."); len(components) == 3 && strings.EqualFold(components[2], "com") && strings.EqualFold(components[1], "medium") && !strings.EqualFold(components[0], "www") {
		username = fmt.Sprintf("@%s", components[0])
	} else {
		return in
	}

	// construct new URL with medium.com host and username path segment
	out = &url.URL{}
	*out = *in
	out.Host = "medium.com"
	out.Path = path.Join("/", username, strings.TrimPrefix(in.Path, "/"))
	out.RawPath = ""
	return out
}
