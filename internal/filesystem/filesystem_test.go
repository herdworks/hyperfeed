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

func TestNewAndReady(t *testing.T) {
	testRootURI := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	fs := &FileSystem{}
	if err := fs.Configure(testRootURI); err != nil {
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
	testRootURI := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	fs := &FileSystem{}
	if err := fs.Configure(testRootURI); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	target := *testRootURI
	target.Path = path.Join(testRootURI.Path, "example.com", "item", "asset.bin")
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
	testRootURI := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	fs := &FileSystem{}
	if err := fs.Configure(testRootURI); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	target := *testRootURI
	target.Path = path.Join(testRootURI.Path, "example.com", "item", "asset.bin")
	ctx := context.Background()
	lockURI := target
	lockURI.Path += ".lock"
	token, err := fs.lock(ctx, &lockURI)
	if err != nil {
		t.Fatalf("Lock error: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token")
	}
	if _, err := fs.lock(ctx, &lockURI); !errors.Is(err, ErrAdvisoryLock) {
		t.Fatalf("expected ErrAdvisoryLock, got %v", err)
	}
	if err := fs.unlock(ctx, &lockURI, "wrong"); !errors.Is(err, ErrInvalidLockToken) {
		t.Fatalf("expected ErrInvalidLockToken, got %v", err)
	}
	if err := fs.unlock(ctx, &lockURI, token); err != nil {
		t.Fatalf("Unlock error: %v", err)
	}
}

func TestEvict(t *testing.T) {
	testRootURI := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	fs := &FileSystem{}
	if err := fs.Configure(testRootURI); err != nil {
		t.Fatalf("Configure error: %v", err)
	}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	create := func(rel string, mod time.Time) {
		target := *testRootURI
		target.Path = path.Join(testRootURI.Path, rel)
		if err := fs.Put(context.Background(), &target, []byte("data")); err != nil {
			t.Fatalf("Put error: %v", err)
		}
		local := filepath.Join(testRootURI.Path, filepath.FromSlash(rel))
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

	oldTarget := *testRootURI
	oldTarget.Path = path.Join(testRootURI.Path, "example.com", "itemA", "asset.bin")
	exists, err := fs.Exists(context.Background(), &oldTarget)
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Fatalf("expected old file to be evicted")
	}

	newTarget := *testRootURI
	newTarget.Path = path.Join(testRootURI.Path, "example.com", "itemB", "fresh.bin")
	exists, err = fs.Exists(context.Background(), &newTarget)
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Fatalf("fresh file should remain")
	}
}

func TestSecurityScope(t *testing.T) {
	var testRootURI *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + string(os.PathSeparator)}
	var fs *FileSystem = &FileSystem{}
	var err error
	tests := []struct {
		name string
		uri  *url.URL
		err  error
	}{
		{
			name: "in scope",
			uri:  &url.URL{Path: filepath.Join(testRootURI.Path, "file.bin")},
			err:  nil,
		},
		{
			name: "out of scope",
			uri:  &url.URL{Path: "/tmp/file.bin"},
			err:  ErrSecurityScope,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err = fs.Configure(testRootURI); err != nil {
				t.Fatalf("Configure error: %v", err)
			}

			// check expected errors
			if err := fs.Put(context.Background(), tc.uri, []byte("data")); tc.err != nil && err == nil {
				t.Fatalf("expected error, did not get one")
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("different error than expected:\ngot:    %v\nwanted: %v", err, tc.err)
			} else {
				return // skip remaining tests on expected error
			}

			// handle unexpected errors
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPruneRemovesEmptyDirectories(t *testing.T) {
	var testRootURI *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	var fs *FileSystem = &FileSystem{}
	var directories []string
	var files []string
	var remaining []string
	var want []string
	var err error
	if err = fs.Configure(testRootURI); err != nil {
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
