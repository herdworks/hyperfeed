package cache

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

type CacheFS interface {
	// Ready returns true if the CacheFS is properly configured with a root URI.
	Ready() bool

	// State/Put/Get/Remove provide basic CRUD operations for cache blobs.
	Stat(uri *url.URL) (exists bool, err error)
	Put(ctx *context.Context, uri *url.URL, data []byte) error
	Get(ctx *context.Context, uri *url.URL) (data []byte, err error)
	Remove(ctx *context.Context, uri *url.URL) error

	// Lock/Unlock provide simple advisory locking for cache blobs.
	Lock(ctx *context.Context, uri *url.URL) (token string, err error)
	Unlock(ctx *context.Context, uri *url.URL, token string) error

	// Evict implements retention-policy removal of expired cache blobs.
	Evict(ctx *context.Context, expiry *time.Time) (int64, error)
}

type Cache struct {
	FS        *CacheFS // underlying CacheFS implementation
	URI       *url.URL // absolute URI for the cache root directory
	Namespace Namespace
}

type Namespace struct {
	Domain string
	Feed   string
}

type File struct {
	Cache     *Cache
	Item      *hyperfeed.Item
	Name      string
}

type Shard struct {
	Year  int
	Month int
	Day   int
}

func (cache *Cache) Ready() bool {
	if cache == nil {
		return false
	}
	if cache.URI == nil {
		return false
	}
	if cache.Namespace.Domain == "" || cache.Namespace.Feed == "" {
		return false
	}
	return true
}

func (namespace *Namespace) String() string {
	return fmt.Sprintf("%s/%s", namespace.Domain, namespace.Feed)
}

func (file *File) Ready() bool {
	if file == nil || file.Cache == nil || file.Item == nil {
		fmt.Printf("file, cache, or item is nil\n")
		return false
	}
	if !file.Cache.Ready() {
		fmt.Printf("cache is not ready\n")
	}
	if file.Item.ID == "" {
		fmt.Printf("cache file missing item ID\n")
		return false
	}
	if file.Item.Published.IsZero() && file.Item.Updated.IsZero() {
		fmt.Printf("cache file missing item timestamp\n")
		return false
	}
	if file.Name == "" {
		fmt.Printf("cache file missing filename\n")
		return false
	}
	return true
}

func (file *File) Shard() (string, error) {
	if !file.Ready() {
		return "", fmt.Errorf("cache file not ready")
	}
	ts := file.Item.Updated
	if ts.IsZero() {
		ts = file.Item.Published
	}
	yyyy := fmt.Sprintf("%04d", ts.UTC().Year())
	mm := fmt.Sprintf("%02d", ts.UTC().Month())
	dd := fmt.Sprintf("%02d", ts.UTC().Day())
	return filepath.Join(yyyy, mm, dd), nil
}

func (file *File) URI() (uri *url.URL, err error) {
	var shard string
	if file == nil || !file.Ready() {
		return nil, fmt.Errorf("cache file is not ready")
	}
	if shard, err = file.Shard(); err != nil {
		return nil, err
	}
	path := filepath.Join(shard, file.Item.ID, file.Name)
	uri = file.Cache.URI.ResolveReference(&url.URL{ Path: path })
	return uri, nil
}
