package hyperfeed

import (
    "net/url"
)

// Author represents a content author or contributor.
type Author struct {
    Name  string   `json:"name,omitempty"`
    Email string   `json:"email,omitempty"`
    URI   *url.URL `json:"uri,omitempty"`
}

// Image represents an image associated with a feed or item.
type Image struct {
    URL    *url.URL `json:"url,omitempty"`
    Title  string   `json:"title,omitempty"`
    Width  int      `json:"width,omitempty"`
    Height int      `json:"height,omitempty"`
}

// Enclosure represents a media attachment (audio, video, etc.).
type Enclosure struct {
    URL    *url.URL `json:"url,omitempty"`
    Length int64    `json:"length,omitempty"`
    Type   string   `json:"type,omitempty"`
}

