// Package reader provides scheme-dispatched byte reading for feed and OPML
// sources. A single *http.Client handles http, https, AND file URLs: file://
// (and empty-scheme paths, via Normalize) read from the local filesystem with
// stdlib conditional-read semantics (If-Modified-Since vs mtime -> 304,
// Last-Modified from mtime, 404 on missing files), while http/https read
// remotely.
//
// Swift interop: URL(fileURLWithPath: "/Users/x/f.opml").absoluteString yields
// "file:///Users/x/f.opml" (empty host, percent-encoded path). url.Parse
// decodes the path, and the file transport opens the decoded path, so Swift
// file URLs round-trip through this package correctly.
package reader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed/href"
	"github.com/herdworks/hyperfeed/version"
)

// TYPES
// ============================================================================

// Config captures tunable knobs for reader behavior.
type Config struct {
	// Timeout specifies the default timeout for read requests (including local file reads).
	Timeout time.Duration

	// MaxContentLength caps the number of bytes Read will buffer in memory for a single response.
	// Reads exceeding this limit return ErrContentLength. Zero means unlimited.
	MaxContentLength int64
}

// VARIABLES
// ============================================================================

// errors
var (
	ErrRead          = errors.New("read error")
	ErrNotFound      = errors.New("not found")
	ErrContentLength = errors.New("content length limit exceeded")
)

var DefaultConfig = Config{
	Timeout:          10 * time.Second,
	MaxContentLength: 4 << 20, // 4 MiB
}

// FUNCTIONS
// ============================================================================

// maxRedirects caps redirect following. The security-critical control is the
// cross-scheme guard below, not the count; this matches Go's default of 10.
const maxRedirects = 10

// Client returns an *http.Client whose transport dispatches by URL scheme:
// http and https read remotely; file reads from the local filesystem rooted
// at "/" (absolute paths map directly; see Normalize).
//
// Because the same client can read both schemes, redirects are constrained to
// http/https targets: a remote server must not be able to redirect a fetch
// into the file transport (which would turn any subscribed feed/OPML server
// into an arbitrary local-file read primitive). The initial request is never a
// redirect, so intentional top-level file:// reads are unaffected.
func Client(config Config) (client *http.Client) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	return &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if scheme := req.URL.Scheme; scheme != "http" && scheme != "https" {
				return fmt.Errorf("%w: refusing redirect to non-web scheme %q", ErrRead, scheme)
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("%w: stopped after %d redirects", ErrRead, maxRedirects)
			}
			return nil
		},
	}
}

// Normalize converts a raw URL-or-path string into an absolute *href.Href
// suitable for Client/Read:
//   - bare or relative filesystem paths -> "file" scheme with an absolute path
//   - file scheme with a relative path -> absolute path (host is preserved as-is)
//   - http/https URLs -> returned unchanged
//
// Inputs without a recognized URL scheme are treated as filesystem paths and
// are NOT run through url.Parse, which would otherwise mangle path characters
// that are legal in filenames but meaningful in URLs ('%', '#', '?'). Setting
// url.URL{Path: abs} preserves the raw path bytes; u.String() percent-encodes
// them so the file transport decodes back to the original path.
func Normalize(raw string) (u *href.Href, err error) {
	if raw == "" {
		return nil, errors.Join(ErrRead, errors.New("cannot normalize empty href"))
	}
	if !hasURLScheme(raw) {
		var abs string
		if abs, err = filepath.Abs(raw); err != nil {
			return nil, errors.Join(ErrRead, err)
		}
		return (*href.Href)(&url.URL{Scheme: "file", Path: abs}), nil
	}
	var parsed *url.URL
	if parsed, err = url.Parse(raw); err != nil {
		return nil, errors.Join(ErrRead, err)
	}
	if parsed.Scheme == "file" && !filepath.IsAbs(parsed.Path) {
		// the file transport ignores the host and requires an absolute path
		var abs string
		if abs, err = filepath.Abs(parsed.Path); err != nil {
			return nil, errors.Join(ErrRead, err)
		}
		parsed.Path = abs
	}
	return (*href.Href)(parsed), nil
}

// hasURLScheme reports whether raw begins with a URL scheme this package
// dispatches on. Prefix matching (rather than url.Parse) avoids misclassifying
// a filesystem path that contains a colon (e.g. "notes:2024.opml") as a URL.
func hasURLScheme(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "file:")
}

// Read performs a one-shot GET against u using client and returns the
// response body. Responses with status >= 400 return an error (missing local
// files surface as ErrNotFound via the file transport's 404). Bodies larger
// than config.MaxContentLength return ErrContentLength.
func Read(ctx context.Context, client *http.Client, u *href.Href, config Config) (body []byte, err error) {
	var request *http.Request
	var response *http.Response
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = Client(config)
	}
	if u == nil {
		return nil, errors.Join(ErrRead, errors.New("cannot read nil href"))
	}
	if request, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil); err != nil {
		return nil, errors.Join(ErrRead, err)
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", version.LibUserAgent)
	}
	if response, err = client.Do(request); err != nil {
		return nil, errors.Join(ErrRead, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, errors.Join(ErrRead, ErrNotFound, fmt.Errorf("read %q: HTTP %d %s", u.String(), response.StatusCode, http.StatusText(response.StatusCode)))
	}
	if response.StatusCode >= 400 {
		return nil, errors.Join(ErrRead, fmt.Errorf("read %q: HTTP %d %s", u.String(), response.StatusCode, http.StatusText(response.StatusCode)))
	}
	var r io.Reader = response.Body
	if config.MaxContentLength > 0 {
		r = io.LimitReader(response.Body, config.MaxContentLength+1) // +1 to detect overflow
	}
	if body, err = io.ReadAll(r); err != nil {
		return nil, errors.Join(ErrRead, err)
	}
	if config.MaxContentLength > 0 && int64(len(body)) > config.MaxContentLength {
		return nil, errors.Join(ErrRead, ErrContentLength, fmt.Errorf("read %q: body exceeds %d bytes", u.String(), config.MaxContentLength))
	}
	return body, nil
}
