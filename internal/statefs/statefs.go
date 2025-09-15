package statefs

import (
	"context"
	"net/url"
)

// StateFileSystem is a stub implementation of state.Store backed by the filesystem.
type StateFileSystem struct {
	URI *url.URL
}

func (s *StateFileSystem) Ready() bool {
	if s == nil {
		return false
	}
	if s.URI == nil {
		return false
	}
	return true
}

func (s *StateFileSystem) Get(ctx context.Context, uri *url.URL) ([]byte, error) {
	return nil, nil
}

func (s *StateFileSystem) Put(ctx context.Context, uri *url.URL, data []byte) error {
	return nil
}

func (s *StateFileSystem) Delete(ctx context.Context, uri *url.URL) error {
	return nil
}

func (s *StateFileSystem) Lock(ctx context.Context, uri *url.URL) (string, error) {
	return "", nil
}

func (s *StateFileSystem) Unlock(ctx context.Context, uri *url.URL, token string) error {
	return nil
}
