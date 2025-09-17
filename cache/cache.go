package cache

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/cachefs"
	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

type CacheFS interface {
	// Ready returns true if the CacheFS is properly configured with a root URI.
	Ready() bool
	Configure(root *url.URL) error
	Init() error

	// Stat/Put/Get/Remove provide basic CRUD operations for cache blobs.
	Stat(ctx context.Context, uri *url.URL) (info os.FileInfo, err error)
	Exists(ctx context.Context, uri *url.URL) (bool, error)
	Put(ctx context.Context, uri *url.URL, data []byte) error
	Get(ctx context.Context, uri *url.URL) (data []byte, err error)
	Remove(ctx context.Context, uri *url.URL) error
	RemoveAll(ctx context.Context, uri *url.URL) error
	Destroy(ctx context.Context) error

	// Lock/Unlock provide simple advisory locking for cache blobs.
	Lock(ctx context.Context, uri *url.URL) (token string, err error)
	Unlock(ctx context.Context, uri *url.URL, token string) error

	// Evict implements retention-policy removal of expired cache blobs.
	Evict(ctx context.Context, expiry *time.Time) (int64, error)
	Prune(ctx context.Context, dir string, allowlist ...string) error
}

type Cache struct {
	FS        CacheFS  // underlying CacheFS implementation
	Base      *url.URL // base URI for all caches
	URI       *url.URL // root URI for this cache (i.e. filepath.Join(Base.Path, Namespace.Domain, Namespace.Feed))
	Namespace Namespace
}

type Namespace struct {
	Domain string
	Feed   string
}

type File struct {
	Cache *Cache
	Item  *hyperfeed.Item
	Name  string
	URI   *url.URL // cached file URI, if previously computed
}

func (cache *Cache) Ready() bool {
	return cache != nil &&
		cache.Base != nil &&
		cache.URI != nil &&
		cache.Namespace.Domain != "" &&
		cache.Namespace.Feed != "" &&
		cache.FS.Ready()
}

// Exists() checks if the cache root directory exists
func (cache *Cache) Exists(ctx context.Context) (bool, error) {
	if !cache.Ready() {
		return false, errors.New("cache not ready")
	}
	return cache.FS.Exists(ctx, cache.URI)
}

// Configure() configures the cache
func (cache *Cache) Configure(base *url.URL, feed *hyperfeed.Feed, hrefs ...string) (err error) {
	if cache == nil || base == nil {
		return errors.New("cache.Configure: one or more configuration parameters are nil")
	}
	if !strings.HasSuffix(base.Path, "/") {
		return errors.New("cache.Configure: base path must have trailing slash")
	}
	if feed == nil && len(hrefs) != 1 {
		return fmt.Errorf("cache.Configure: feed or href is required (%d hrefs provided)", len(hrefs))
	}
	if feed == nil {
		feed = &hyperfeed.Feed{}
		feed.URL, err = url.Parse(hrefs[0])
		if err != nil {
			return fmt.Errorf("cache.Configure: parse feed href: %w", err)
		}
	}
	if feed.URL == nil {
		return errors.New("cache.Configure: feed.URL is required for cache configuration")
	}
	if feed.Domain() == "" {
		return fmt.Errorf("cache.Configure: feed.URL %q could not produce domain", feed.URL.String())
	}
	cache.Base = base
	cache.Namespace = Namespace{Domain: feed.Domain(), Feed: feed.Identifier()}
	cache.URI = &url.URL{Path: filepath.Join(cache.Base.Path, "", feed.Domain(), feed.Identifier(), "")} // the "" element ensures trailing slash
	cache.FS = &cachefs.FileSystem{}
	return cache.FS.Configure(cache.URI)
}

// Create() ensures the cache root directory exists
func (cache *Cache) Create() error {
	if !cache.Ready() {
		return errors.New("cache: cache not ready")
	}
	if err := cache.FS.Init(); err != nil {
		return fmt.Errorf("cache: creating root path: %w", err)
	}
	return nil
}

// Destroy() removes the cache root directory and all contents
func (cache *Cache) Destroy(ctx context.Context) error {
	if !cache.Ready() {
		return errors.New("cache.Destroy: cache not ready")
	}
	if err := cache.FS.Destroy(ctx); err != nil {
		return fmt.Errorf("cache.Destroy: error destroying root path: %w", err)
	}
	return nil
}

// Purge() resets the cache by performing Destroy() and Create()
func (cache *Cache) Purge(ctx context.Context) error {
	if err := cache.Destroy(ctx); err != nil {
		return fmt.Errorf("cache.Purge: destroy: %w", err)
	}
	if err := cache.Create(); err != nil {
		return fmt.Errorf("cache.Purge: create: %w", err)
	}
	return nil
}

func (namespace *Namespace) String() string {
	return fmt.Sprintf("%s/%s", namespace.Domain, namespace.Feed)
}

func (file *File) Ready() (ready bool, err error) {
	if !file.Cache.Ready() {
		return false, errors.New("cache.File: invalid cache")
	}
	if !file.Item.Ready() {
		return false, errors.New("cache.File: invalid item")
	}
	if file.URI == nil {
		return false, errors.New("cache.File: unresolved file URI")
	}
	if file.Name, err = normalized.CleanPath(file.Name); err != nil {
		return false, fmt.Errorf("cache.File: invalid filename: %w", err)
	}
	return true, nil
}

// Exists() checks if the file exists
func (file *File) Exists(ctx context.Context) (bool, error) {
	if exists, err := file.Ready(); !exists || err != nil {
		return false, errors.New("cache not ready")
	}
	return file.Cache.FS.Exists(ctx, file.URI)
}

// Shard returns the YYYY/MM/DD shard for the file using the item's timestamp.
func (file *File) Shard() (string, error) {
	if file.Item == nil {
		return "", errors.New("file.Shard: item is nil")
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

// RelativePath returns the filesystem path rooted at the cache namespace.
func (file *File) RelativePath() (p string, err error) {
	var shard string
	var name string
	if shard, err = file.Shard(); err != nil {
		return "", err
	}
	if name, err = normalized.CleanPath(file.Name); err != nil {
		return "", fmt.Errorf("cache.File.RelativePath: invalid filename: %w", err)
	}
	return filepath.Join(shard, file.Item.Identifier(), name), nil
}

// URI returns the fully-qualified cache URI for the file.
func (file *File) ResolvedURI() (u *url.URL, err error) {
	var relativePath string
	if file.URI != nil {
		return file.URI, nil
	}
	if relativePath, err = file.RelativePath(); err != nil {
		return nil, err
	}
	file.URI = &url.URL{Path: path.Join(file.Cache.URI.Path, filepath.ToSlash(relativePath))}
	return file.URI, nil
}

// Put stores data at the cache file URI, acquiring a lock if provided by the FS implementation.
func (file *File) Put(ctx context.Context, data []byte) (err error) {
	if ready, err := file.Ready(); !ready {
		return err
	}
	return file.Cache.FS.Put(ctx, file.URI, data)
}

// Get retrieves data from the cache file URI.
func (file *File) Get(ctx context.Context) ([]byte, error) {
	if ready, err := file.Ready(); !ready {
		return nil, err
	}
	return file.Cache.FS.Get(ctx, file.URI)
}

// Remove deletes the cache blob for the file.
func (file *File) Remove(ctx context.Context) error {
	if ready, err := file.Ready(); !ready {
		return err
	}
	return file.Cache.FS.Remove(ctx, file.URI)
}

// Stat reports file information
func (file *File) Stat(ctx context.Context) (os.FileInfo, error) {
	if ready, err := file.Ready(); !ready {
		return nil, err
	}
	return file.Cache.FS.Stat(ctx, file.URI)
}

// Lock acquires an advisory lock for the cache file.
func (file *File) Lock(ctx context.Context) (string, error) {
	if ready, err := file.Ready(); !ready {
		return "", err
	}
	return file.Cache.FS.Lock(ctx, file.URI)
}

// Unlock releases an advisory lock for the cache file.
func (file *File) Unlock(ctx context.Context, token string) error {
	if ready, err := file.Ready(); !ready {
		return err
	}
	return file.Cache.FS.Unlock(ctx, file.URI, token)
}

// Evict removes expired cache blobs using the underlying FS implementation.
func (cache *Cache) Evict(ctx context.Context, expiry *time.Time) (int64, error) {
	if !cache.Ready() {
		return 0, errors.New("cache: cache not ready")
	}
	return cache.FS.Evict(ctx, expiry)
}
