package state

import "context"

// Namespace identifies a logical state domain (e.g., subscriber or scheduler).
type Namespace string

const (
	NamespaceSubscriber Namespace = "subscriber"
	NamespaceScheduler  Namespace = "scheduler"
)

// Store provides get/put/delete and simple advisory locks for state blobs.
// Keys are implementation-defined paths within a namespace (e.g., "subscriptions.json",
// "scheduler.json", "feeds/<feedID>.json").
type Store interface {
	Get(ctx context.Context, ns Namespace, key string) ([]byte, error)
	Put(ctx context.Context, ns Namespace, key string, data []byte) error
	Delete(ctx context.Context, ns Namespace, key string) error

	// Lock returns a token that must be passed to Unlock. Implementations should
	// create a best-effort lock (e.g., a .lock file). Locks are advisory.
	Lock(ctx context.Context, ns Namespace, key string) (token string, err error)
	Unlock(ctx context.Context, ns Namespace, key string, token string) error
}
