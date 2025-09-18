package state

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

type StateFS interface {
	// Ready returns true if the StateFS is properly configured with a root URI.
	Ready() bool
	Configure(root *url.URL, policies ...filesystem.LockPolicy) error
	Init() error

	// Stat/Put/Remove provide basic CUD operations for cache blobs (cache is only responsible for writing & eviction, not reads)
	Stat(ctx context.Context, uri *url.URL) (info os.FileInfo, err error)
	Exists(ctx context.Context, uri *url.URL) (bool, error)
	Get(ctx context.Context, uri *url.URL) (data []byte, err error)
	Put(ctx context.Context, uri *url.URL, data []byte) error
	Remove(ctx context.Context, uri *url.URL) error
	RemoveAll(ctx context.Context, uri *url.URL) error
	Destroy(ctx context.Context) error

	// Evict implements retention-policy removal of expired cache blobs.
	Evict(ctx context.Context, expiry *time.Time) (int64, error)
	Prune(ctx context.Context, dir string, allowlist ...string) error
}

var (
	ErrConfiguration = errors.New("state: invalid configuration")
	ErrReadiness     = errors.New("state: state not ready")
	ErrInvalidFile   = errors.New("state: invalid file")
)

type State struct {
	FS        StateFS  // underlying StateFS implementation
	Base      *url.URL // base URI for all state stores
	URI       *url.URL // root URI for this state store (i.e. filepath.Join(Base.Path, Namespace.Domain, Namespace.Feed))
	Namespace Namespace
}

type Namespace struct {
	Domain string
	Feed   string
}

type File struct {
	State *State
	Name  string
	URI   *url.URL // cached file URI, if previously computed
}

// ============================= State Methods ================================

// Configure() configures the state
func (state *State) Configure(base *url.URL, feed *hyperfeed.Feed, hrefs ...string) (err error) {
	if state == nil || base == nil {
		return errors.Join(ErrConfiguration, errors.New("one or more configuration parameters are nil"))
	}
	if !strings.HasSuffix(base.Path, string(os.PathSeparator)) {
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
	state.Base = base
	state.Namespace = Namespace{Domain: feed.Domain(), Feed: feed.Identifier()}
	state.URI = &url.URL{Path: filepath.Join(state.Base.Path, "", feed.Domain(), feed.Identifier(), "")} // the "" element ensures trailing slash
	state.FS = &filesystem.FileSystem{}
	return state.FS.Configure(state.URI, state.Policies()...)
}

func (state *State) Policies() []filesystem.LockPolicy {
	return []filesystem.LockPolicy{
		{
			// Lock the state filesystem (i.e. **.json)
			Name:    "State Files",
			Pattern: regexp.MustCompile(`\.json$`),
			Operations: []filesystem.OperationKind{
				filesystem.CreateOperation,
				filesystem.DeleteOperation,
			},
		},
	}
}

// Ready() returns true if the state store is properly configured
func (state *State) Ready() bool {
	return state != nil &&
		state.Base != nil &&
		state.URI != nil &&
		state.Namespace.Domain != "" &&
		state.Namespace.Feed != "" &&
		state.FS.Ready()
}

// Exists() checks if the cache root directory exists
func (state *State) Exists(ctx context.Context) (bool, error) {
	if !state.Ready() {
		return false, ErrReadiness
	}
	return state.FS.Exists(ctx, state.URI)
}

// Create() ensures the cache root directory exists
func (state *State) Create() error {
	if !state.Ready() {
		return ErrReadiness
	}
	if err := state.FS.Init(); err != nil {
		return fmt.Errorf("cache: creating root path: %w", err)
	}
	return nil
}

// Destroy() removes the cache root directory and all contents
func (state *State) Destroy(ctx context.Context) error {
	if !state.Ready() {
		return errors.New("state.Destroy: cache not ready")
	}
	if err := state.FS.Destroy(ctx); err != nil {
		return fmt.Errorf("state.Destroy: error destroying root path: %w", err)
	}
	return nil
}

// Purge() resets the cache by performing Destroy() and Create()
func (state *State) Purge(ctx context.Context) error {
	if err := state.Destroy(ctx); err != nil {
		return fmt.Errorf("state.Purge: destroy: %w", err)
	}
	if err := state.Create(); err != nil {
		return fmt.Errorf("state.Purge: create: %w", err)
	}
	return nil
}

// Evict removes expired cache blobs using the underlying FS implementation.
func (state *State) Evict(ctx context.Context, expiry *time.Time) (int64, error) {
	if !state.Ready() {
		return 0, ErrReadiness
	}
	return state.FS.Evict(ctx, expiry)
}

// ============================= Namespace Methods ============================

// String() returns a string representation of the namespace
func (namespace *Namespace) String() string {
	return fmt.Sprintf("%s/%s", namespace.Domain, namespace.Feed)
}

// ============================= File Methods =================================

// Ready() returns true if the cache file is properly configured
func (file *File) Ready() (ready bool, err error) {
	if !file.State.Ready() {
		return false, errors.Join(ErrInvalidFile, errors.New("invalid file state"))
	}
	if !strings.HasSuffix(file.Name, ".json") {
		file.Name += ".json"
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
	file.URI = &url.URL{Path: path.Join(file.State.URI.Path, relativePath)}
	return file.URI, nil
}

// RelativePath returns the filesystem path rooted at the cache namespace.
func (file *File) RelativePath() (p string, err error) {
	var name string
	if name, err = normalized.CleanPath(file.Name); err != nil {
		return "", errors.Join(ErrInvalidFile, fmt.Errorf("cache.File.RelativePath: invalid filename: %w", err))
	}
	return filepath.Join(name), nil
}

// Exists() checks if the file exists
func (file *File) Exists(ctx context.Context) (bool, error) {
	if exists, err := file.Ready(); !exists || err != nil {
		return false, errors.New("state not ready")
	}
	return file.State.FS.Exists(ctx, file.URI)
}

// Put stores data at the cache file URI, acquiring a lock if provided by the FS implementation.
func (file *File) Put(ctx context.Context, data []byte) (err error) {
	if ready, err := file.Ready(); !ready {
		return err
	}
	return file.State.FS.Put(ctx, file.URI, data)
}

// Remove deletes the cache blob for the file.
func (file *File) Remove(ctx context.Context) error {
	if ready, err := file.Ready(); !ready {
		return err
	}
	return file.State.FS.Remove(ctx, file.URI)
}

// Stat reports file information
func (file *File) Stat(ctx context.Context) (os.FileInfo, error) {
	if ready, err := file.Ready(); !ready {
		return nil, err
	}
	return file.State.FS.Stat(ctx, file.URI)
}
