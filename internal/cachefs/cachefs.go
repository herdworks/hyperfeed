package cachefs

import (
	"context"
	sterrors "errors"
	"io"

	"github.com/herdworks/hyperfeed-go/cache"
)

// Options configure the filesystem cache.
type Options struct {
	// BaseDir is the root directory for cached objects and indexes.
	BaseDir string
}

// New creates a new filesystem-backed cache.
func New(opts Options) (cache.Cache, error) {
	return &fsCache{baseDir: opts.BaseDir}, nil
}

type fsCache struct {
	baseDir string
}

var errNotImplemented = sterrors.New("cachefs: not implemented")

func (c *fsCache) Put(ctx context.Context, key cache.Key, r io.Reader, meta cache.Meta) (cache.Location, error) {
	return cache.Location{}, errNotImplemented
}

func (c *fsCache) Get(ctx context.Context, key cache.Key) (io.ReadCloser, cache.Meta, error) {
	return nil, cache.Meta{}, errNotImplemented
}

func (c *fsCache) Stat(ctx context.Context, key cache.Key) (cache.Meta, error) {
	return cache.Meta{}, errNotImplemented
}

func (c *fsCache) Remove(ctx context.Context, key cache.Key) error {
	return errNotImplemented
}

func (c *fsCache) Evict(ctx context.Context, policy cache.Policy) error {
	return errNotImplemented
}
