package transporter

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Transporter is a placeholder for future expansion where we might
// support advanced behaviors (retries, backoff, caching, auth, etc.).
// For now, functionality is exposed via package-level helpers.
type Transporter struct {
	Client  *http.Client
	Options *TransporterOptions
}

func NewTransporter() *Transporter {
	t := &Transporter{}
	t.Init()
	return t
}

// TransporterOptions holds configuration options for a Transporter.
type TransporterOptions struct {
	// Timeout specifies the timeout for HTTP requests.
	// If zero, a default of 15 seconds is used.
	Timeout time.Duration

	// UserAgent specifies the User-Agent header to use for requests.
	// If empty, a default User-Agent is used.
	UserAgent string

	// MinStatusCode and MaxStatusCode specify the acceptable range of HTTP status codes.
	// If zero, the default range is 200-299.
	MinStatusCode int
	MaxStatusCode int
}

// TransporterRequestOptions holds options for individual requests.
type TransporterRequestOptions struct {
	// Headers specifies additional headers to include in the request.
	Headers map[string]string
}

// Ready returns true if the Transporter is properly configured for use.
func (transporter *Transporter) Ready() bool {
	if transporter.Options == nil {
		return false
	}
	if transporter.Client == nil {
		return false
	}
	return true
}

// Init initializes the Transporter with default settings if not already set.
func (transporter *Transporter) Init() {
	if transporter.Options == nil {
		transporter.Options = &TransporterOptions{}
		transporter.Options.Timeout = 15 * time.Second
		transporter.Options.UserAgent = "hyperfeed-go/transporter (+https://github.com/herdworks/hyperfeed-go)"
		transporter.Options.MinStatusCode = 200
		transporter.Options.MaxStatusCode = 299
	}
	if transporter.Client == nil {
		transporter.Client = &http.Client{
			Timeout: transporter.Options.Timeout,
		}
	}
}

// Get fetches the content at the given href using the package HTTP client.
func (t *Transporter) Get(href string) (body []byte, err error) {
	var request *http.Request
	var response *http.Response
	if href == "" {
		return nil, errors.New("href is required")
	}
	if request, err = t.Request(http.MethodGet, href, nil); err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if response, err = t.Client.Do(request); err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < t.Options.MinStatusCode || response.StatusCode > t.Options.MaxStatusCode {
		return nil, fmt.Errorf("unexpected status: %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

// Request generates a request for the given URL and options
func (t *Transporter) Request(method string, href string, headers map[string]string) (req *http.Request, err error) {
	if href == "" {
		return nil, errors.New("href is required")
	}
	if method == "" {
		method = http.MethodGet
	}
	req = &http.Request{}
	req.Method = method
	req.Header = http.Header{}
	req.Header.Set("User-Agent", t.Options.UserAgent)
	req.URL, err = url.ParseRequestURI(href)
	if err != nil {
		return nil, fmt.Errorf("invalid href: %w", err)
	}
	return req, nil
}
