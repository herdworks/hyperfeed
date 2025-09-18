package filesystem

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newFileURI(t *testing.T, dir string) *url.URL {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	u, err := url.Parse("file://" + filepath.ToSlash(abs))
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}
	return u
}

func TestNewAndReady(t *testing.T) {
	dir := t.TempDir()
	root := newFileURI(t, dir)
	fs := &FileSystem{}
	if err := fs.Configure(root); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if !fs.Ready() {
		t.Fatalf("filesystem should report ready")
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}
}

func TestPutGetStatRemove(t *testing.T) {
	dir := t.TempDir()
	root := newFileURI(t, dir)
	fs := &FileSystem{}
	if err := fs.Configure(root); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	target := *root
	target.Path = path.Join(root.Path, "example.com", "item", "asset.bin")
	lockURI := target
	lockURI.Path += ".lock"

	ctx := context.Background()
	payload := []byte("hello cache")

	if err := fs.Put(ctx, &target, payload); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	exists, err := fs.Exists(ctx, &target)
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Fatalf("expected file to exist")
	}

	data, err := fs.Get(ctx, &target)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("Get data mismatch: %q", data)
	}

	if err := fs.Remove(ctx, &target); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	exists, err = fs.Exists(ctx, &target)
	if err != nil {
		t.Fatalf("Exists after remove error: %v", err)
	}
	if exists {
		t.Fatalf("expected file to be removed")
	}
}

func TestLockUnlock(t *testing.T) {
	dir := t.TempDir()
	root := newFileURI(t, dir)
	fs := &FileSystem{}
	if err := fs.Configure(root); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	target := *root
	target.Path = path.Join(root.Path, "example.com", "item", "asset.bin")
	ctx := context.Background()
	lockURI := target
	lockURI.Path += ".lock"

	token, err := fs.Lock(ctx, &lockURI)
	if err != nil {
		t.Fatalf("Lock error: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token")
	}

	if _, err := fs.Lock(ctx, &lockURI); !errors.Is(err, ErrLocked) {
		t.Fatalf("expected ErrLocked, got %v", err)
	}

	if err := fs.Unlock(ctx, &lockURI, "wrong"); !errors.Is(err, ErrLockToken) {
		t.Fatalf("expected ErrLockToken, got %v", err)
	}

	if err := fs.Unlock(ctx, &lockURI, token); err != nil {
		t.Fatalf("Unlock error: %v", err)
	}
}

func TestEvict(t *testing.T) {
	dir := t.TempDir()
	root := newFileURI(t, dir)
	fs := &FileSystem{}
	if err := fs.Configure(root); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	create := func(rel string, mod time.Time) {
		target := *root
		target.Path = path.Join(root.Path, rel)
		if err := fs.Put(context.Background(), &target, []byte("data")); err != nil {
			t.Fatalf("Put error: %v", err)
		}
		local := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.Chtimes(local, mod, mod); err != nil {
			t.Fatalf("Chtimes error: %v", err)
		}
	}

	oldTime := time.Now().Add(-48 * time.Hour)
	newTime := time.Now()
	create("example.com/itemA/asset.bin", oldTime)
	create("example.com/itemB/fresh.bin", newTime)

	cutoff := time.Now().Add(-24 * time.Hour)
	removed, err := fs.Evict(context.Background(), &cutoff)
	if err != nil {
		t.Fatalf("Evict error: %v", err)
	}
	if removed == 0 {
		t.Fatalf("expected some bytes removed")
	}

	oldTarget := *root
	oldTarget.Path = path.Join(root.Path, "example.com", "itemA", "asset.bin")
	exists, err := fs.Exists(context.Background(), &oldTarget)
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Fatalf("expected old file to be evicted")
	}

	newTarget := *root
	newTarget.Path = path.Join(root.Path, "example.com", "itemB", "fresh.bin")
	exists, err = fs.Exists(context.Background(), &newTarget)
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Fatalf("fresh file should remain")
	}
}

func TestSecurityScope(t *testing.T) {
	dir := t.TempDir()
	root := newFileURI(t, dir)
	fs := &FileSystem{}
	if err := fs.Configure(root); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	other, _ := url.Parse("file:///tmp/other/file.bin")
	if err := fs.Put(context.Background(), other, []byte("data")); !errors.Is(err, ErrSecurityScope) {
		t.Fatalf("expected ErrSecurityScope, got %v", err)
	}
}

func TestPruneRemovesEmptyDirectories(t *testing.T) {
	var fs *FileSystem = &FileSystem{}
	var directories []string
	var files []string
	var remaining []string
	var want []string
	var err error
	if err = fs.Configure(&url.URL{Path: "testdata"}); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Destroy(context.Background()); err != nil {
		t.Fatalf("Purge error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	directories = []string{
		filepath.Join(fs.URI.Path, "example.com", "blog", "hello-world"),
		filepath.Join(fs.URI.Path, "example.com", "blog", "hello-again"),
		filepath.Join(fs.URI.Path, "example.com", "thoughts-on-prune"),
	}
	files = []string{
		filepath.Join(directories[0], ".DS_Store"),
		filepath.Join(directories[1], "index.html"),
		filepath.Join(directories[2], ".DS_Store"),
		filepath.Join(directories[2], "index.html"),
		filepath.Join(directories[2], "cover.png"),
	}
	want = []string{
		filepath.Join(fs.URI.Path, "example.com"),
		filepath.Join(fs.URI.Path, "example.com", "blog"),
		filepath.Join(fs.URI.Path, "example.com", "blog", "hello-again"),
		filepath.Join(fs.URI.Path, "example.com", "thoughts-on-prune"),
	}
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll error: %v", err)
		}
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}
	}
	if err := fs.Prune(context.Background(), fs.URI.Path); err != nil {
		t.Fatalf("Prune error: %v", err)
	}
	if remaining, err = fs.inventory(fs.URI.Path, os.ModeDir); err != nil {
		t.Fatalf("inventory error: %v", err)
	}
	for _, d := range remaining {
		if !slices.Contains(want, d) {
			t.Errorf("unexpected remaining directory: %q", d)
		}
	}

}
