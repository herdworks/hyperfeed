package hyperfeed

import "net/url"

// ============================ Package Types =================================

type Author struct {
	Name    string   `json:"name,omitempty"`
	URL     *url.URL `json:"url,omitempty"`
	Email   string   `json:"email,omitempty"`
	Favicon *url.URL `json:"avatar,omitempty"`
}
