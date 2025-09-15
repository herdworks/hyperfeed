package state

import (
	"context"
	"net/url"
)

// StateFS provides get/put/delete and simple advisory locks for state blobs.
// Keys are implementation-defined URIs.
type StateFS interface {
	// Ready returns true if the StateFS is properly configured with a root URI.
	Ready() bool

	// Get/Put/Delete provide basic CRUD operations for state blobs.
	Get(ctx context.Context, uri *url.URL) ([]byte, error)
	Put(ctx context.Context, uri *url.URL, data []byte) error
	Delete(ctx context.Context, uri *url.URL) error

	// Lock returns a token that must be passed to Unlock. Implementations should
	// create a best-effort lock (e.g., a .lock file). Locks are advisory.
	Lock(ctx context.Context, uri *url.URL) (token string, err error)
	Unlock(ctx context.Context, uri *url.URL, token string) error
}
