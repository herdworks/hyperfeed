package cache

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

var cacheTime = time.Now()

func TestCacheConfigure(t *testing.T) {
	cacheRoot := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	tests := []struct {
		name string
		feed *hyperfeed.Feed
		href string
		want *Cache
		err  error
	}{
		{
			name: "from feed",
			feed: &hyperfeed.Feed{
				URL: &href.Href{Scheme: "https", Host: "example.com", Path: "/news/"},
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
			err: nil,
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
			err: nil,
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
			err: nil,
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
			err: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cache *Cache = &Cache{}
			var err error
			if err = cache.Configure(cacheRoot, tc.feed, tc.href); tc.err != nil && errors.Is(err, tc.err) {
				return // got expected error
			} else if err != nil {
				t.Errorf("unexpected error:\n\twanted: %v\n\tgot:    %v", tc.err, err)
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
	var cacheRoot *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	var cache *Cache = &Cache{}
	var err error
	tests := []struct {
		name string
		href string
		want string
	}{
		{
			name: "from href with path",
			href: "https://example.com/blog",
			want: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/blog")),
		},
		{
			name: "from href with root path",
			href: "https://example.com/",
			want: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/")),
		},
		{
			name: "from href with trailing slash",
			href: "https://example.com/news/",
			want: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/news")),
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
	var cacheRoot *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	var cache *Cache = &Cache{}
	var err error
	tests := []struct {
		name string
		href string
		want string
	}{
		{
			name: "from href with root path",
			href: "https://example.com/",
			want: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/")),
		},
		{
			name: "from href with path",
			href: "https://example.com/feed",
			want: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/feed")),
		},
		{
			name: "from href with trailing slash",
			href: "https://example.com/news/",
			want: filepath.Join(cacheRoot.Path, "example.com", normalized.Identifier("/news")),
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
	cacheRoot := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	feeds := []*hyperfeed.Feed{
		{
			URL: &href.Href{Scheme: "https", Host: "example.com", Path: "/"},
		},
		{
			URL: &href.Href{Scheme: "https", Host: "example.com", Path: "/blog/"},
		},
	}
	items := []*hyperfeed.Item{
		{
			URL:         &url.URL{Scheme: "https", Host: "example.com", Path: "/thoughts-on-hashtags/"},
			Feed:        feeds[0],
			PublishedAt: cacheTime,
		},
		{
			URL:         &url.URL{Scheme: "https", Host: "example.com", Path: "/blog/hello-again/"},
			Feed:        feeds[1],
			PublishedAt: cacheTime,
		},
	}
	tests := []struct {
		name     string
		feed     hyperfeed.Feed
		item     hyperfeed.Item
		fileName string
		err      error
	}{
		{
			name:     "with invalid file path",
			feed:     *feeds[0],
			item:     *items[1],
			fileName: "../cover.png",
			err:      ErrInvalidFile,
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
			var fileBytes []byte
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
					cache.URI.Path,                           // cache namespace root
					tc.item.PublishedAt.UTC().Format("2006"), // year shard
					tc.item.PublishedAt.UTC().Format("01"),   // month shard
					tc.item.PublishedAt.UTC().Format("02"),   // day shard
					tc.item.Identifier(),                     // item.URL hash
					tc.fileName,
				),
			}

			// init cache file
			file = &File{
				Cache: cache,
				Item:  &tc.item,
				Name:  tc.fileName,
			}
			if _, err = file.Ready(); tc.err != nil && errors.Is(err, tc.err) {
				return // skip remaining tests on expected error
			} else if err != nil {
				t.Fatalf("file.Ready unexpected error: %v", err)
			}
			if file.URI.Path != want.Path {
				t.Fatalf("cache.File URI does not match expected path;\ngot:    %q\nwanted: %q", file.URI.Path, want.Path)
			}
			if fileBytes, err = os.ReadFile(filepath.Join("testdata", filepath.Base(file.URI.Path))); tc.err != nil && errors.Is(err, tc.err) {
				return // skip remaining tests on expected error
			} else if err != nil {
				t.Fatalf("reading testdata/cover.png: %v", err)
			}
			if err := file.Put(ctx, fileBytes); tc.err != nil && errors.Is(err, tc.err) {
				return // skip remaining tests on expected error
			} else if err != nil {
				t.Fatalf("Put error: %v", err)
			}
			if exists, err := file.Exists(ctx); !exists || err != nil {
				t.Fatalf("file.Exists failed (%v, %v)", exists, err)
			}
		})
	}
}
