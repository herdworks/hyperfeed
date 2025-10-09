package cache

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed-go/internal/filesystem"
	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ============================= Package Types ================================

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

type CacheFS interface {
	// Ready returns true if the CacheFS is properly configured with a root URI.
	Ready() bool
	Configure(root *url.URL, policies ...filesystem.LockPolicy) error
	Init() error

	// Stat/Put/Remove provide basic CUD operations for cache blobs (cache is only responsible for writing & eviction, not reads)
	Stat(ctx context.Context, uri *url.URL) (info os.FileInfo, err error)
	Exists(ctx context.Context, uri *url.URL) (bool, error)
	Put(ctx context.Context, uri *url.URL, data []byte) error
	Remove(ctx context.Context, uri *url.URL) error
	RemoveAll(ctx context.Context, uri *url.URL) error
	Destroy(ctx context.Context) error

	// Evict implements retention-policy removal of expired cache blobs.
	Evict(ctx context.Context, expiry *time.Time) (int64, error)
	Prune(ctx context.Context, dir string, allowlist ...string) error
}

// ============================= Package Variables ============================
var (
	ErrConfiguration = errors.New("cache: invalid configuration")
	ErrReadiness     = errors.New("cache: cache not ready")
	ErrInvalidFile   = errors.New("cache: invalid file")
)

// ============================= Cache Methods ================================

// Configure() configures the cache
func (cache *Cache) Configure(base *url.URL, feed *hyperfeed.Feed, hrefs ...string) (err error) {
	if cache == nil || base == nil {
		return errors.Join(ErrConfiguration, errors.New("one or more configuration parameters are nil"))
	}
	if !strings.HasSuffix(base.Path, "/") {
		return errors.Join(ErrConfiguration, errors.New("base path must have trailing slash"))
	}
	if feed == nil && len(hrefs) != 1 {
		return errors.Join(ErrConfiguration, fmt.Errorf("feed or href is required (%d hrefs provided)", len(hrefs)))
	}
	if feed == nil {
		feed = &hyperfeed.Feed{}
		feed.URL, err = url.Parse(hrefs[0])
		if err != nil {
			return errors.Join(ErrConfiguration, fmt.Errorf("failed to parse feed href; %w", err))
		}
	}
	if feed.URL == nil {
		return errors.Join(ErrConfiguration, errors.New("feed.URL is required for cache configuration"))
	}
	if feed.Domain() == "" {
		return errors.Join(ErrConfiguration, fmt.Errorf("invalid domain for feed.URL %q", feed.URL.String()))
	}
	cache.Base = base
	cache.Namespace = Namespace{Domain: feed.Domain(), Feed: fmt.Sprintf("%s", feed.PathIdentifier())}
	cache.URI = &url.URL{Path: filepath.Join(cache.Base.Path, "", feed.Domain(), fmt.Sprintf("%s", feed.PathIdentifier()), "")} // the "" element ensures trailing slash
	cache.FS = &filesystem.FileSystem{}
	return cache.FS.Configure(cache.URI, cache.Policies()...)
}

func (cache *Cache) Policies() []filesystem.LockPolicy {
	return []filesystem.LockPolicy{
		{
			// Lock the item cache (i.e. *YYYY/MM/DD/:itemID/*)
			Name:    "Item Cache",
			Pattern: regexp.MustCompile(`\d{4}/\d{2}/\d{2}/.*/.*`),
			Operations: []filesystem.OperationKind{
				filesystem.CreateOperation,
				filesystem.DeleteOperation,
				filesystem.DeleteAllOperation,
			},
		},
	}
}

// Ready() returns true if the cache is properly configured
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
		return false, ErrReadiness
	}
	return cache.FS.Exists(ctx, cache.URI)
}

// Create() ensures the cache root directory exists
func (cache *Cache) Create() error {
	if !cache.Ready() {
		return ErrReadiness
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

// Evict removes expired cache blobs using the underlying FS implementation.
func (cache *Cache) Evict(ctx context.Context, expiry *time.Time) (int64, error) {
	if !cache.Ready() {
		return 0, ErrReadiness
	}
	return cache.FS.Evict(ctx, expiry)
}

// ============================= Namespace Methods ============================

// String() returns a string representation of the namespace
func (namespace *Namespace) String() string {
	return fmt.Sprintf("%s/%s", namespace.Domain, namespace.Feed)
}

// ============================= File Methods =================================

// Ready() returns true if the cache file is properly configured
func (file *File) Ready() (ready bool, err error) {
	if !file.Item.Ready() {
		return false, errors.Join(ErrInvalidFile, errors.New("invalid file item"))
	}
	if !file.Cache.Ready() {
		return false, errors.Join(ErrInvalidFile, errors.New("invalid file cache"))
	}
	if _, err = file.ResolvedURI(); err != nil {
		return false, errors.Join(ErrInvalidFile, fmt.Errorf("failed to resolve file URI; %w", err))
	}
	if file.Name, err = normalized.CleanPath(file.Name); err != nil {
		return false, errors.Join(ErrInvalidFile, fmt.Errorf("invalid filename: %w", err))
	}
	return true, nil
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

// RelativePath returns the filesystem path rooted at the cache namespace.
func (file *File) RelativePath() (p string, err error) {
	var shard string
	var name string
	if shard, err = file.Shard(); err != nil {
		return "", err
	}
	if name, err = normalized.CleanPath(file.Name); err != nil {
		return "", errors.Join(ErrInvalidFile, fmt.Errorf("cache.File.RelativePath: invalid filename: %w", err))
	}
	return filepath.Join(shard, fmt.Sprintf("%s", file.Item.Identifier()), name), nil
}

// Exists() checks if the file exists
func (file *File) Exists(ctx context.Context) (bool, error) {
	if exists, err := file.Ready(); !exists || err != nil {
		return false, errors.New("cache not ready")
	}
	return file.Cache.FS.Exists(ctx, file.URI)
}

// Shard() returns the YYYY/MM/DD shard for the file using the item's timestamp.
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

// Put stores data at the cache file URI, acquiring a lock if provided by the FS implementation.
func (file *File) Put(ctx context.Context, data []byte) (err error) {
	if ready, err := file.Ready(); !ready {
		return err
	}
	return file.Cache.FS.Put(ctx, file.URI, data)
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
