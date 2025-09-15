package cachefs

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

type CacheFileSystem struct {
	URI *url.URL // absolute URI for the cache root directory
}

var ErrorSecurityScope error = fmt.Errorf("cachefs: security scope violation")

func (c *CacheFileSystem) Ready() bool {
	if c == nil {
		return false
	}
	if c.URI == nil {
		return false
	}
	return true
}

// check if a key is outside the scope of the cache root directory
// if key is an absolute URL and the path starts with the cache root path, return true
// if key is a relative URL with no scheme, return true
// otherwise, return false
func (c *CacheFileSystem) SecurityScoped(key *url.URL) bool {
	if c == nil || c.URI == nil || key == nil {
		return false
	}
	if key.IsAbs() && key.Scheme == c.URI.Scheme && key.Host == c.URI.Host && strings.HasPrefix(key.Path, c.URI.Path) {
		return true
	}
	if !key.IsAbs() && !strings.HasPrefix(key.Path, "/") {
		return true
	}
	return false
}

func (c *CacheFileSystem) Put(ctx context.Context, uri *url.URL, r io.Reader) error {
	if !c.SecurityScoped(uri) {
		return ErrorSecurityScope
	}
	return nil
}

func (c *CacheFileSystem) Get(ctx context.Context, uri *url.URL) (io.ReadCloser, error) {
	if !c.SecurityScoped(uri) {
		return nil, ErrorSecurityScope
	}
	return nil, nil
}

func (c *CacheFileSystem) Stat(ctx context.Context, uri *url.URL) (os.FileInfo, error) {
	if !c.SecurityScoped(uri) {
		return nil, ErrorSecurityScope
	}
	return nil, nil
}

func (c *CacheFileSystem) Remove(ctx context.Context, uri *url.URL) error {
	if !c.SecurityScoped(uri) {
		return ErrorSecurityScope
	}
	return nil
}

func (c *CacheFileSystem) Evict(ctx context.Context, expiry *time.Time) (byteCount int64, err error) {
	return 0, nil
}
