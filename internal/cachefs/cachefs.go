package cachefs

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var (
	ErrSecurityScope = errors.New("cachefs: security scope violation")
	ErrLocked        = errors.New("cachefs: lock already acquired")
	ErrLockToken     = errors.New("cachefs: invalid lock token")
)

// FileSystem provides a filesystem-backed implementation of cache.CacheFS.
type FileSystem struct {
	URI *url.URL
}

// Ready reports whether the filesystem has a configured root.
func (fs *FileSystem) Ready() bool {
	return fs != nil && fs.URI != nil && fs.URI.Path != ""
}

// Configure sets the filesystem root namespace without creating directories.
func (fs *FileSystem) Configure(root *url.URL) (err error) {
	if root == nil {
		return fmt.Errorf("cachefs: root uri is required")
	}
	if root.Scheme != "file" && root.Scheme != "" {
		return fmt.Errorf("cachefs: unsupported scheme %q", root.Scheme)
	}
	if root.Path == "" {
		return fmt.Errorf("cachefs: invalid cache root path")
	}
	abs := *root // shallow copy
	if abs.Path, err = filepath.Abs(abs.Path); err != nil {
		return fmt.Errorf("cachefs: failed to resolve absolute path: %v", err)
	}
	if !strings.HasSuffix(abs.Path, "/") {
		abs.Path += "/"
	}
	fs.URI = &abs
	return nil
}

// Init() ensures the root path exists (via os.MkdirAll).
func (fs *FileSystem) Init() error {
	if !fs.Ready() {
		return fmt.Errorf("cachefs: filesystem not ready")
	}
	return os.MkdirAll(fs.URI.Path, 0o755)
}

func (fs *FileSystem) SecurityScopedURI(uri *url.URL) (u *url.URL, err error) {
	u = &url.URL{}
	if !fs.Ready() {
		return nil, fmt.Errorf("cachefs: filesystem not ready")
	}
	if uri == nil {
		return nil, fmt.Errorf("cachefs: uri is required")
	}
	if !(uri.Scheme == "file" || uri.Scheme == "") {
		return nil, fmt.Errorf("cachefs: unsupported uri scheme %q", uri.Scheme)
	}
	if u.Path, err = filepath.Abs(uri.Path); err != nil {
		return nil, fmt.Errorf("cachefs: failed to resolve absolute path: %v", err)
	}
	if !strings.HasPrefix(u.Path, strings.TrimSuffix(fs.URI.Path, "/")) {
		return nil, fmt.Errorf("cachefs: uri %q is outside of filesystem scope %q: %w", u.Path, fs.URI.String(), ErrSecurityScope)
	}
	return u, nil
}

func (fs *FileSystem) Stat(ctx context.Context, uri *url.URL) (info os.FileInfo, err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return nil, fmt.Errorf("cachefs.Stat error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.Stat(secURI.Path)
}

func (fs *FileSystem) Exists(ctx context.Context, uri *url.URL) (bool, error) {
	_, err := fs.Stat(ctx, uri)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (fs *FileSystem) Put(ctx context.Context, uri *url.URL, data []byte) (err error) {
	var tempfile *os.File
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("cachefs.Stat error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(secURI.Path), 0o755); err != nil {
		return err
	}
	if tempfile, err = os.CreateTemp(filepath.Dir(secURI.Path), ".cache-"); err != nil {
		return err
	}
	if _, err := tempfile.Write(data); err != nil {
		tempfile.Close()
		os.Remove(tempfile.Name())
		return err
	}
	if err := tempfile.Sync(); err != nil {
		tempfile.Close()
		os.Remove(tempfile.Name())
		return err
	}
	if err := tempfile.Close(); err != nil {
		os.Remove(tempfile.Name())
		return err
	}
	if err := os.Rename(tempfile.Name(), secURI.Path); err != nil {
		os.Remove(tempfile.Name())
		return err
	}
	return nil
}

func (fs *FileSystem) Get(ctx context.Context, uri *url.URL) (b []byte, err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return nil, fmt.Errorf("cachefs.Get error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(secURI.Path)
}

func (fs *FileSystem) Remove(ctx context.Context, uri *url.URL) (err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("cachefs.Remove error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(secURI.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (fs *FileSystem) RemoveAll(ctx context.Context, uri *url.URL) (err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("cachefs.RemoveAll error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.RemoveAll(secURI.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Destroy() removes the cache root directory and all its contents
func (fs *FileSystem) Destroy(ctx context.Context) error {
	if !fs.Ready() {
		return fmt.Errorf("cachefs.Destroy: filesystem not ready")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.RemoveAll(fs.URI.Path)
}

func (fs *FileSystem) Lock(ctx context.Context, uri *url.URL) (token string, err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return "", fmt.Errorf("cachefs.Lock error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(secURI.Path), 0o755); err != nil {
		return "", err
	}
	lockPath := filepath.Join(filepath.Dir(secURI.Path), "cache.lock")
	token = fmt.Sprintf("%d", time.Now().UnixNano())
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", ErrLocked
		}
		return "", err
	}
	if _, err := file.WriteString(token); err != nil {
		file.Close()
		os.Remove(lockPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(lockPath)
		return "", err
	}
	return token, nil
}

func (fs *FileSystem) Unlock(ctx context.Context, uri *url.URL, token string) (err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("cachefs.Unlock error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	lockPath := filepath.Join(filepath.Dir(secURI.Path), "cache.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(string(data)) != token {
		return ErrLockToken
	}
	return os.Remove(lockPath)
}

func (fs *FileSystem) Evict(ctx context.Context, expiry *time.Time) (int64, error) {
	if expiry == nil {
		return 0, nil
	}
	cutoff := expiry.UTC()
	var removed int64
	err := filepath.WalkDir(fs.URI.Path, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "cache.lock" {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().UTC().Before(cutoff) {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			removed += info.Size()
			fs.Prune(ctx, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return removed, err
	}
	return removed, nil
}

// Recuse over the cache tree and remove "prunable" directories
func (fs *FileSystem) Prune(ctx context.Context, dir string, allowlist ...string) (err error) {
	var directories []string
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(allowlist) == 0 {
		allowlist = []string{".DS_Store"}
	}
	// recurse over the cache tree to discover all directories
	if directories, err = fs.inventory(dir, os.ModeDir); err != nil {
		return err
	}
	// iterate discovered directories in reverse order (deepest first)
	slices.Reverse(directories)
	for _, directory := range directories {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !fs.prunable(directory, allowlist) {
			continue
		}
		if err = os.RemoveAll(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// inventory recurses over a directory tree contents and returns a slice of entries
func (fs *FileSystem) inventory(directory string, filters ...os.FileMode) ([]string, error) {
	var entries []string
	walkErr := filepath.WalkDir(directory, func(path string, entry iofs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if path == directory {
			return nil
		}
		if len(filters) > 0 && slices.Contains(filters, entry.Type()) {
			entries = append(entries, path)
		}
		return nil
	})
	if walkErr != nil {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil, nil
		}
		return nil, walkErr
	}
	return entries, nil
}

// prunable checks directory contents to determine if a directory is safe to prune
func (fs *FileSystem) prunable(directory string, allowlist []string) bool {
	var entries []os.DirEntry
	var err error
	if len(allowlist) == 0 {
		allowlist = []string{".DS_Store"}
	}
	if entries, err = os.ReadDir(directory); err != nil {
		return errors.Is(err, os.ErrNotExist)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return false
		}
		if slices.Contains(allowlist, filepath.Base(entry.Name())) {
			continue
		}
		return false
	}
	return true
}
