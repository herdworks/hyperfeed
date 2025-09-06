package cache

import (
	"context"
	"io"
	"time"
)

// Key identifies cached content; implementations may use content hashes or
// normalized URLs to derive keys.
type Key struct {
	Namespace string // e.g., "image", "enclosure"
	ID        string // e.g., content hash or normalized URL
}

// Meta provides lightweight metadata for cached content.
type Meta struct {
	Size        int64
	ContentType string
	CreatedAt   time.Time
	LastAccess  time.Time
}

// Policy configures eviction decisions.
type Policy struct {
	MaxBytes      int64
	MaxItemAge    time.Duration
	ProtectPinned bool
}

// Location optionally describes where content is stored (path, URI, etc.).
type Location struct {
	Path string
	URI  string
}

// Cache defines the asset cache interface.
type Cache interface {
	Put(ctx context.Context, key Key, r io.Reader, meta Meta) (Location, error)
	Get(ctx context.Context, key Key) (rc io.ReadCloser, meta Meta, err error)
	Stat(ctx context.Context, key Key) (Meta, error)
	Remove(ctx context.Context, key Key) error
	Evict(ctx context.Context, policy Policy) error
}
