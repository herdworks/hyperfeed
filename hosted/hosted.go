package hosted

import (
	"net/url"
	"strings"

	"github.com/herdworks/hyperfeed/hosted/medium.com"
)

func URL(in *url.URL) (out *url.URL) {
	switch {
	case strings.Contains(strings.ToLower(in.Hostname()), "medium.com"):
		return medium.URL(in)
	default:
		return in
	}
}
