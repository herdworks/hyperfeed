package filesystem

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	ErrSecurityScope = errors.New("filesystem: security scope violation")
	ErrLocked        = errors.New("filesystem: lock already acquired")
	ErrLockToken     = errors.New("filesystem: invalid lock token")
)

// FileSystem provides a filesystem-backed implementation that operates beneath
// higher-level packages such as cache or state.
type FileSystem struct {
	URI             *url.URL
	LockingPolicies []LockPolicy
}

// LockPolicy indicates the advisory locking policy of the filesystem.
type LockPolicy struct {
	Pattern    *regexp.Regexp
	Operations []OperationKind
}

// OperationKind indicates the type of filesystem operation being requested.
//
// NOTES:
//
// * a "Prune" is a ListOperation + RemoveAllOperation
// * an "Exists" is a StatOperation
// * an "Update" is a GetOperation + PutOperation
type OperationKind int

const (
	StatOperation      OperationKind = iota
	PutOperation                     // Create
	GetOperation                     // Read
	RemoveOperation                  // Delete
	RemoveAllOperation               // Recursive Delete
	ListOperation                    // List
)

var SudoOperation = []OperationKind{
	StatOperation,
	PutOperation,
	GetOperation,
	RemoveOperation,
	ListOperation,
	RemoveAllOperation,
}

// ============================ FileSystem Management =========================

// Configure sets the filesystem root namespace without creating directories.
func (fs *FileSystem) Configure(root *url.URL) (err error) {
	if root == nil {
		return fmt.Errorf("filesystem: root uri is required")
	}
	if root.Scheme != "file" && root.Scheme != "" {
		return fmt.Errorf("filesystem: unsupported scheme %q", root.Scheme)
	}
	if root.Path == "" {
		return fmt.Errorf("filesystem: invalid root path")
	}
	abs := *root // shallow copy
	if abs.Path, err = filepath.Abs(abs.Path); err != nil {
		return fmt.Errorf("filesystem: failed to resolve absolute path: %v", err)
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
		return fmt.Errorf("filesystem: not ready")
	}
	return os.MkdirAll(fs.URI.Path, 0o755)
}

// Ready reports whether the filesystem has a configured root.
// TODO: return (bool, error) with details on why the filesystem isn't ready
func (fs *FileSystem) Ready() bool {
	// FileSystem must have a valid root URI
	if !(fs != nil && fs.URI != nil && fs.URI.Path != "") {
		return false
	}
	// FileSystem policies must be valid (if defined)
	for _, policy := range fs.LockingPolicies {
		if policy.Pattern == nil || len(policy.Operations) == 0 {
			return false
		}
	}
	return true
}

// SecurityScopedURI checks that a URI is within the FileSyStem security scope and returns an absolute path
func (fs *FileSystem) SecurityScopedURI(uri *url.URL) (u *url.URL, err error) {
	u = &url.URL{}
	if !fs.Ready() {
		return nil, fmt.Errorf("filesystem.SecurityScopedURI error: filesystem not ready")
	}
	if uri == nil {
		return nil, fmt.Errorf("filesystem.SecurityScopedURI error: URI is required")
	}
	if !(uri.Scheme == "file" || uri.Scheme == "") {
		return nil, fmt.Errorf("filesystem.SecurityScopedURI error: unsupported URI scheme %q", uri.Scheme)
	}
	if u.Path, err = filepath.Abs(uri.Path); err != nil {
		return nil, fmt.Errorf("filesystem.SecurityScopedURI error: failed to resolve URI absolute path: %v", err)
	}
	if !strings.HasPrefix(u.Path, strings.TrimSuffix(fs.URI.Path, "/")) {
		return nil, fmt.Errorf("filesystem.SecurityScopedURI error: requested URI breaches the security scope;\nrequested       %q:\nsecurity scope: %q", u.Path, fs.URI.String())
	}
	return u, nil
}

// AdvisoryLockURI checks if a URI operation requires an exclusive lock and returns a lockfile URI if so
func (fs *FileSystem) AdvisoryLockURI(uri *url.URL, ops ...OperationKind) (u *url.URL, err error) {
	var abspath string
	var subpath string
	if !fs.Ready() {
		return nil, fmt.Errorf("filesystem.AdvisoryLockURI: not ready")
	}
	if len(ops) == 0 {
		ops = SudoOperation // default to sudo mode (all operations)
	}
	if abspath, err = filepath.Abs(uri.Path); err != nil {
		return nil, fmt.Errorf("filesystem.AdvisoryLockedURI: failed to resolve absolute path: %v", err)
	} else {
		subpath = strings.TrimPrefix(abspath, strings.TrimSuffix(fs.URI.Path, string(os.PathSeparator)))
		subpath = strings.TrimPrefix(subpath, string(os.PathSeparator))
	}
	if subpath == "" {
		return nil, fmt.Errorf("filesystem.AdvisoryLockedURI: cannot lock the root path")
	}
	for _, policy := range fs.LockingPolicies {
		if policy.LockRequired(&url.URL{Path: subpath}, ops...) {
			abspath = strings.TrimSuffix(abspath, string(os.PathSeparator))
			abspath = strings.TrimSuffix(abspath, ".lock")
			abspath += ".lock"
			return &url.URL{Path: abspath}, nil
		}
	}
	return nil, nil
}

// LockRequired checks if a URI operation requires an exclusive lock based a given policy
func (policy *LockPolicy) LockRequired(uri *url.URL, ops ...OperationKind) bool {
	if policy == nil || uri == nil {
		return false
	}
	if policy.Pattern == nil {
		return false
	}
	if !policy.Pattern.MatchString(uri.Path) {
		return false
	}
	for _, op := range ops {
		if slices.Contains(policy.Operations, op) {
			return true
		}
	}
	return false
}

// Lock() creates a lockfile at the given URI and returns a token to be used for unlocking
func (fs *FileSystem) Lock(ctx context.Context, uri *url.URL) (t string, err error) {
	var secURI *url.URL
	var lockfile *os.File
	var token = fmt.Sprintf("%d", time.Now().UnixNano())
	if uri == nil {
		return "", nil
	}
	if !strings.HasSuffix(uri.Path, ".lock") {
		return "", fmt.Errorf("filesystem.Lock error: expected lock uri to end with .lock")
	}
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return "", fmt.Errorf("filesystem.Lock error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(secURI.Path), 0o755); err != nil {
		return "", err
	}
	if lockfile, err = os.OpenFile(secURI.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", ErrLocked
		}
		return "", err
	}
	if _, err := lockfile.WriteString(token); err != nil {
		lockfile.Close()
		os.Remove(secURI.Path)
		return "", err
	}
	if err := lockfile.Close(); err != nil {
		os.Remove(secURI.Path)
		return "", err
	}
	return token, nil
}

// Unlock() removes a lockfile at the given URI if the provided token matches
func (fs *FileSystem) Unlock(ctx context.Context, uri *url.URL, token string) (err error) {
	var secURI *url.URL
	var data []byte
	if uri == nil {
		return nil
	}
	if !strings.HasSuffix(uri.Path, ".lock") {
		return fmt.Errorf("filesystem.Unlock error: expected lock uri to end with .lock")
	}
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("filesystem.Unlock error: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if data, err = os.ReadFile(secURI.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(string(data)) != token {
		return ErrLockToken
	}
	return os.Remove(secURI.Path)
}

// ============================ FileSystem Operations =========================

// Stat() returns os.FileInfo about a file or directory at the given URI
func (fs *FileSystem) Stat(ctx context.Context, uri *url.URL) (info os.FileInfo, err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return nil, fmt.Errorf("filesystem.Stat: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.Stat(secURI.Path)
}

// Exists() checks if a file or directory exists at the given URI
func (fs *FileSystem) Exists(ctx context.Context, uri *url.URL) (exists bool, err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return false, fmt.Errorf("filesystem.Stat: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if _, err := fs.Stat(ctx, secURI); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Put() writes data to a file at the given URI, creating parent directories as needed
func (fs *FileSystem) Put(ctx context.Context, uri *url.URL, data []byte) (err error) {
	var secURI *url.URL
	var lockURI *url.URL
	var tempfile *os.File
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("filesystem.Put error: access denied: %w", err)
	}
	if lockURI, err = fs.AdvisoryLockURI(uri, StatOperation); err != nil {
		return fmt.Errorf("filesystem.Put error: advisory lock checks failed: %w", err)
	}
	if token, err := fs.Lock(ctx, lockURI); err != nil {
		return err
	} else {
		// defer unlock if a lock is obtained
		defer fs.Unlock(ctx, lockURI, token)
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

// Get() reads data from a file at the given URI
func (fs *FileSystem) Get(ctx context.Context, uri *url.URL) (b []byte, err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return nil, fmt.Errorf("filesystem.Get: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(secURI.Path)
}

// Remove() deletes a file at the given URI
func (fs *FileSystem) Remove(ctx context.Context, uri *url.URL) (err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("filesystem.Remove: access denied: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(secURI.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveAll() recursively deletes a directory tree at the given URI
func (fs *FileSystem) RemoveAll(ctx context.Context, uri *url.URL) (err error) {
	var secURI *url.URL
	if secURI, err = fs.SecurityScopedURI(uri); err != nil {
		return fmt.Errorf("filesystem.RemoveAll: access denied: %w", err)
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
		return fmt.Errorf("filesystem.Destroy error: not ready")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.RemoveAll(fs.URI.Path)
}

// Evict() removes files older than the given expiry time and prunes empty directories
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
		if len(filters) == 0 {
			entries = append(entries, path)
		} else if slices.Contains(filters, entry.Type()) {
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

// List recurses over a directory tree contents and returns a slice of entries
// func (fs *FileSystem) List(ctx context.Context, directory *url.URL, filters ...os.FileMode) (entries []os.DirEntry, err error) {
// 	var secURI *url.URL
// 	if secURI, err = fs.SecurityScopedURI(directory); err != nil {
// 		return nil, fmt.Errorf("filesystem.Unlock: access denied: %w", err)
// 	}
// 	if err := ctx.Err(); err != nil {
// 		return nil, err
// 	}
// 	walkErr := filepath.WalkDir(secURI.Path, func(path string, entry iofs.DirEntry, err error) error {
// 		if err != nil {
// 			if errors.Is(err, os.ErrNotExist) {
// 				return nil
// 			}
// 			return err
// 		}
// 		if path == secURI.Path {
// 			return nil
// 		}
// 		if len(filters) == 0 {
// 			entries = append(entries, entry)
// 		} else if slices.Contains(filters, entry.Type()) {
// 			entries = append(entries, entry)
// 		}
// 		return nil
// 	})
// 	if walkErr != nil {
// 		if errors.Is(walkErr, os.ErrNotExist) {
// 			return nil, nil
// 		}
// 		return nil, walkErr
// 	}
// 	return entries, nil
// }
