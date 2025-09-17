package cache

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

var cacheRootPath = filepath.Join("testdata", "testcache") + "/"
var cacheRoot = &url.URL{Path: cacheRootPath}
var cacheTime = time.Now()

func TestCacheConfigure(t *testing.T) {
	tests := []struct {
		name          string
		feed          *hyperfeed.Feed
		href          string
		want          *Cache
		errorExpected bool
	}{
		{
			name: "from feed",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/news/"},
			},
			href: "",
			want: &Cache{
				Base: cacheRoot,
				URI:  &url.URL{Path: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/news"))},
				Namespace: Namespace{
					Domain: "example.com",
					Feed:   normalized.Identifier("/news"),
				},
			},
			errorExpected: false,
		},
		{
			name: "from href with path",
			feed: nil,
			href: "https://example.com/feed/",
			want: &Cache{
				Base: cacheRoot,
				URI:  &url.URL{Path: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/feed"))},
				Namespace: Namespace{
					Domain: "example.com",
					Feed:   normalized.Identifier("/feed"),
				},
			},
			errorExpected: false,
		},
		{
			name: "from href with root path",
			feed: nil,
			href: "https://example.com/",
			want: &Cache{
				Base: cacheRoot,
				URI:  &url.URL{Path: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/"))},
				Namespace: Namespace{
					Domain: "example.com",
					Feed:   normalized.Identifier("/"),
				},
			},
			errorExpected: false,
		},
		{
			name: "from href with trailing slash",
			feed: nil,
			href: "https://example.com/blog/",
			want: &Cache{
				Base: cacheRoot,
				URI:  &url.URL{Path: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/blog"))},
				Namespace: Namespace{
					Domain: "example.com",
					Feed:   normalized.Identifier("/blog"),
				},
			},
			errorExpected: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cache *Cache = &Cache{}
			var err error
			if tc.errorExpected {
				if err = cache.Configure(cacheRoot, tc.feed, tc.href); err == nil {
					t.Errorf("Configure() expected error, got nil")
				}
				return
			}
			if err = cache.Configure(cacheRoot, tc.feed, tc.href); err != nil {
				t.Errorf("Configure() unexpected error: %v", err)
			}
			if cache.Base.Path != tc.want.Base.Path {
				t.Errorf("cache.Base = %q, want %q", cache.Base.Path, tc.want.Base.Path)
			}
			if cache.URI.Path != tc.want.URI.Path {
				t.Errorf("cache.URI = %q, want %q", cache.URI.Path, tc.want.URI.Path)
			}
			if cache.Namespace != tc.want.Namespace {
				t.Errorf("cache.Namespace = %+v, want %+v", cache.Namespace, tc.want.Namespace)
			}
			if cache.Namespace.Domain != tc.want.Namespace.Domain {
				t.Errorf("cache.Namespace.Domain = %q, want %q", cache.Namespace.Domain, tc.want.Namespace.Domain)
			}
			if cache.Namespace.Feed != tc.want.Namespace.Feed {
				t.Errorf("cache.Namespace.Feed = %q, want %q", cache.Namespace.Feed, tc.want.Namespace.Feed)
			}
			// Report results
			if tc.feed != nil && tc.feed.URL != nil {
				t.Logf("cache configured for feed: %q", tc.feed.URL.String())
				t.Logf("cache configured with URI: %q", cache.URI.Path)
			} else if tc.href != "" {
				t.Logf("cache configured for href: %q", tc.href)
				t.Logf("cache configured with URI: %q", cache.URI.Path)
			}

		})
	}
}

func TestCacheReadiness(t *testing.T) {
	var cache *Cache = &Cache{}
	var err error
	tests := []struct {
		name          string
		href          string
		want          string
		errorExpected bool
	}{
		{
			name:          "from href with path",
			href:          "https://example.com/blog",
			want:          filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/blog")),
			errorExpected: false,
		},
		{
			name:          "from href with root path",
			href:          "https://example.com/",
			want:          filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/")),
			errorExpected: false,
		},
		{
			name:          "from href with trailing slash",
			href:          "https://example.com/news/",
			want:          filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/news")),
			errorExpected: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err = cache.Configure(cacheRoot, nil, tc.href); err != nil {
				t.Fatalf("cache.Configure error: %v", err)
			}
			if cache.URI.Path != tc.want {
				t.Fatalf("cache.URI.Path = %q, want %q", cache.URI.Path, tc.want)
			}
			if !cache.Ready() {
				t.Fatalf("cache not ready")
			} else {
				t.Logf("cache ready for feed: %q", tc.href)
				t.Logf("cache ready with URI: %q", cache.URI.Path)
			}
		})
	}
}

func TestCacheCreateDestroy(t *testing.T) {
	var cache *Cache = &Cache{}
	var err error
	tests := []struct {
		name          string
		href          string
		want          string
		errorExpected bool
	}{
		{
			name:          "from href with root path",
			href:          "https://example.com/",
			want:          filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/")),
			errorExpected: false,
		},
		{
			name:          "from href with path",
			href:          "https://example.com/feed",
			want:          filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/feed")),
			errorExpected: false,
		},
		{
			name:          "from href with trailing slash",
			href:          "https://example.com/news/",
			want:          filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/news")),
			errorExpected: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if err = cache.Configure(cacheRoot, nil, tc.href); err != nil {
				t.Fatalf("cache.Configure error: %v", err)
			} else {
				t.Logf("cache configured from href: %q", tc.href)
				t.Logf("cache configured with URI: %q", cache.URI.Path)
			}
			if err := cache.Destroy(ctx); err != nil {
				t.Fatalf("cache.Destroy error during cleanup: %v", err)
			}
			if exists, err := cache.Exists(ctx); err != nil || exists {
				t.Fatalf("cache.Destroy failed (exists: %v, err: %q)\n", exists, err)
			}
			if err := cache.Create(); err != nil {
				t.Fatalf("cache.Create error: %v", err)
			} else {
				t.Logf("cache created at %q", cache.URI.Path)
			}
			if exists, err := cache.Exists(ctx); err != nil || !exists {
				t.Fatalf("cache.Create failed (exists: %v, err: %q)\n", exists, err)
			}
			if err := cache.Destroy(ctx); err != nil {
				t.Fatalf("cache.Destroy error: %v", err)
			}
			if exists, err := cache.Exists(ctx); err != nil || exists {
				t.Fatalf("cache.Destroy failed, cache still exists")
			} else {
				t.Logf("cache destroyed at %q", cache.URI.Path)
			}
		})
	}
}

func TestCacheFilePutOperation(t *testing.T) {
	feeds := []*hyperfeed.Feed{
		&hyperfeed.Feed{
			URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
		},
		&hyperfeed.Feed{
			URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/blog/"},
		},
	}
	items := []*hyperfeed.Item{
		{
			URL:       &url.URL{Scheme: "https", Host: "example.com", Path: "/blog/hello-world/"},
			Published: cacheTime,
		},
		{
			URL:       &url.URL{Scheme: "https", Host: "example.com", Path: "/blog/hello-again/"},
			Published: cacheTime,
		},
	}
	tests := []struct {
		name          string
		feed          hyperfeed.Feed
		item          hyperfeed.Item
		fileName      string
		errorExpected bool
	}{
		{
			name:          "with invalid file path",
			feed:          *feeds[0],
			item:          *items[1],
			fileName:      "../cover.png",
			errorExpected: true,
		},
		{
			name:     "with valid png file path",
			feed:     *feeds[0],
			item:     *items[0],
			fileName: "cover.png",
		},
		{
			name:     "with valid svg file path",
			feed:     *feeds[1],
			item:     *items[1],
			fileName: "feed-icon.svg",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cache *Cache = &Cache{}
			var file *File
			var err error
			var ctx context.Context = context.Background()

			// setup cache
			if err = cache.Configure(cacheRoot, &tc.feed); err != nil {
				t.Fatalf("cache.Configure error: %v", err)
			}
			if err := cache.Purge(ctx); err != nil {
				t.Fatalf("cache.Purge error: %v", err)
			}

			// setup wanted result
			var want *url.URL = &url.URL{
				Path: filepath.Join(
					cache.URI.Path,                         // cache namespace root
					tc.item.Published.UTC().Format("2006"), // year shard
					tc.item.Published.UTC().Format("01"),   // month shard
					tc.item.Published.UTC().Format("02"),   // day shard
					tc.item.Identifier(),                   // item.URL hash
					tc.fileName,
				),
			}

			// init cache file
			file = &File{
				Cache: cache,
				Item:  &tc.item,
				Name:  tc.fileName,
			}
			file.Name, err = normalized.CleanPath(file.Name)
			if tc.errorExpected && err != nil {
				return
			} else if tc.errorExpected && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if _, err = file.ResolvedURI(); err != nil {
				t.Fatalf("cache.File ResolvedURI error: %v", err)
			}
			if err != nil {
				t.Fatalf("invalid file name %q: %v", file.Name, err)
			}
			if ready, err := file.Ready(); !ready {
				t.Fatalf("file not ready: %v", err)
			}
			if file.URI.Path != want.Path {
				t.Fatalf("cache.File URI does not match expected path;\ngot:    %q\nwanted: %q", file.URI.Path, want.Path)
			}

			fileBytes, err := os.ReadFile(filepath.Join("testdata", filepath.Base(file.URI.Path)))
			if err != nil {
				t.Fatalf("reading testdata/cover.png: %v", err)
			}
			if err := file.Put(ctx, fileBytes); err != nil {
				t.Fatalf("Put error: %v", err)
			}
			if exists, err := file.Exists(ctx); !exists || err != nil {
				t.Fatalf("file.Exists failed (%v, %v)", exists, err)
			}
			// bytes, err := os.ReadFile(localPath)
			// if err != nil {
			// 	t.Fatalf("reading stored file: %v", err)
			// }
			// if string(bytes) != string(payload) {
			// 	t.Fatalf("stored payload mismatch: %q", bytes)
			// }

			// if exists, err := file.Stat(ctx); err != nil || !exists {
			// 	t.Fatalf("Stat expected true, got (%v, %v)", exists, err)
			// }

			// got, err := file.Get(ctx)
			// if err != nil {
			// 	t.Fatalf("Get error: %v", err)
			// }
			// if string(got) != string(payload) {
			// 	t.Fatalf("Get returned %q, want %q", got, payload)
			// }

			// token, err := file.Lock(ctx)
			// if err != nil {
			// 	t.Fatalf("Lock error: %v", err)
			// }
			// lockPath := filepath.Join(filepath.Dir(localPath), "cache.lock")
			// lockBytes, err := os.ReadFile(lockPath)
			// if err != nil {
			// 	t.Fatalf("reading lock file: %v", err)
			// }
			// if strings.TrimSpace(string(lockBytes)) != token {
			// 	t.Fatalf("lock token mismatch")
			// }

			// if err := file.Unlock(ctx, token); err != nil {
			// 	t.Fatalf("Unlock error: %v", err)
			// }
			// if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
			// 	t.Fatalf("lock file should be removed, err=%v", err)
			// }

			// if err := file.Remove(ctx); err != nil {
			// 	t.Fatalf("Remove error: %v", err)
			// }
			// if exists, err := file.Stat(ctx); err != nil || exists {
			// 	t.Fatalf("Stat expected false after remove, got (%v, %v)", exists, err)
			// }
			// if _, err := os.Stat(localPath); !errors.Is(err, os.ErrNotExist) {
			// 	t.Fatalf("expected local file removed, err=%v", err)
			// }

			// // Recreate then evict via retention policy
			// if err := file.Put(ctx, payload); err != nil {
			// 	t.Fatalf("Put error: %v", err)
			// }
			// oldTime := time.Now().Add(-48 * time.Hour)
			// if err := os.Chtimes(localPath, oldTime, oldTime); err != nil {
			// 	t.Fatalf("Chtimes error: %v", err)
			// }
			// expiry := time.Now().Add(-24 * time.Hour)
			// removed, err := cache.Evict(ctx, &expiry)
			// if err != nil {
			// 	t.Fatalf("Evict error: %v", err)
			// }
			// if removed == 0 {
			// 	t.Fatalf("expected eviction to remove bytes")
			// }
			// if exists, err := file.Stat(ctx); err != nil || exists {
			// 	t.Fatalf("file should be evicted, got (%v, %v)", exists, err)
			// }
		})
	}
}
