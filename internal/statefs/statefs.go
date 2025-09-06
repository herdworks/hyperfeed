package statefs

import (
	"context"
	sterrors "errors"

	"github.com/herdworks/hyperfeed-go/state"
)

// Options configure the filesystem store.
type Options struct {
	// BaseDir is the root directory for state files.
	BaseDir string
}

// New creates a new filesystem-backed state store.
func New(opts Options) (state.Store, error) {
	return &storeFS{baseDir: opts.BaseDir}, nil
}

// storeFS is a stub implementation of state.Store backed by the filesystem.
type storeFS struct {
	baseDir string
}

var errNotImplemented = sterrors.New("statefs: not implemented")

func (s *storeFS) Get(ctx context.Context, ns state.Namespace, key string) ([]byte, error) {
	return nil, errNotImplemented
}

func (s *storeFS) Put(ctx context.Context, ns state.Namespace, key string, data []byte) error {
	return errNotImplemented
}

func (s *storeFS) Delete(ctx context.Context, ns state.Namespace, key string) error {
	return errNotImplemented
}

func (s *storeFS) Lock(ctx context.Context, ns state.Namespace, key string) (string, error) {
	return "", errNotImplemented
}

func (s *storeFS) Unlock(ctx context.Context, ns state.Namespace, key string, token string) error {
	return errNotImplemented
}
