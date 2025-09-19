package state

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	normalized "github.com/herdworks/hyperfeed-go/internal/normalization"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

type Example struct {
	Name string `json:"name,omitempty"`
	Age  int    `json:"age,omitempty"`
}
func (*Example) StateFileName() string {
	return "example.json"
}

func TestStateConfigure(t *testing.T) {
	stateRoot := &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	tests := []struct {
		name string
		feed *hyperfeed.Feed
		href string
		want *State
		err  error
	}{
		{
			name: "from feed",
			feed: &hyperfeed.Feed{
				URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/news/"},
			},
			href: "",
			want: &State{
				Base: stateRoot,
				URI:  &url.URL{Path: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/news"))},
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
			want: &State{
				Base: stateRoot,
				URI:  &url.URL{Path: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/feed"))},
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
			want: &State{
				Base: stateRoot,
				URI:  &url.URL{Path: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/"))},
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
			want: &State{
				Base: stateRoot,
				URI:  &url.URL{Path: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/blog"))},
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
			var state *State = &State{}
			var err error
			if err = state.Configure(stateRoot, tc.feed, tc.href); tc.err != nil && err == nil {
				t.Errorf("expected %v, got nil", tc.err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("different error than expected:\ngot:    %v\nwanted: %v", err, tc.err)
			} else {
				return // skip remaining tests on expected error
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if state.Base.Path != tc.want.Base.Path {
				t.Errorf("state.Base = %q, want %q", state.Base.Path, tc.want.Base.Path)
			}
			if state.URI.Path != tc.want.URI.Path {
				t.Errorf("state.URI = %q, want %q", state.URI.Path, tc.want.URI.Path)
			}
			if state.Namespace != tc.want.Namespace {
				t.Errorf("state.Namespace = %+v, want %+v", state.Namespace, tc.want.Namespace)
			}
			if state.Namespace.Domain != tc.want.Namespace.Domain {
				t.Errorf("state.Namespace.Domain = %q, want %q", state.Namespace.Domain, tc.want.Namespace.Domain)
			}
			if state.Namespace.Feed != tc.want.Namespace.Feed {
				t.Errorf("state.Namespace.Feed = %q, want %q", state.Namespace.Feed, tc.want.Namespace.Feed)
			}
			// Report results
			if tc.feed != nil && tc.feed.URL != nil {
				t.Logf("state configured for feed: %q", tc.feed.URL.String())
				t.Logf("state configured with URI: %q", state.URI.Path)
			} else if tc.href != "" {
				t.Logf("state configured for href: %q", tc.href)
				t.Logf("state configured with URI: %q", state.URI.Path)
			}

		})
	}
}

func TestStateReadiness(t *testing.T) {
	var stateRoot *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	var state *State = &State{}
	var err error
	tests := []struct {
		name string
		href string
		want string
	}{
		{
			name: "from href with path",
			href: "https://example.com/blog",
			want: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/blog")),
		},
		{
			name: "from href with root path",
			href: "https://example.com/",
			want: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/")),
		},
		{
			name: "from href with trailing slash",
			href: "https://example.com/news/",
			want: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/news")),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err = state.Configure(stateRoot, nil, tc.href); err != nil {
				t.Fatalf("state.Configure error: %v", err)
			}
			if state.URI.Path != tc.want {
				t.Fatalf("state.URI.Path = %q, want %q", state.URI.Path, tc.want)
			}
			if !state.Ready() {
				t.Fatalf("state not ready")
			} else {
				t.Logf("state ready for feed: %q", tc.href)
				t.Logf("state ready with URI: %q", state.URI.Path)
			}
		})
	}
}

func TestStateCreateDestroy(t *testing.T) {
	var stateRoot *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	var state *State = &State{}
	var err error
	tests := []struct {
		name string
		href string
		want string
	}{
		{
			name: "from href with root path",
			href: "https://example.com/",
			want: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/")),
		},
		{
			name: "from href with path",
			href: "https://example.com/feed",
			want: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/feed")),
		},
		{
			name: "from href with trailing slash",
			href: "https://example.com/news/",
			want: filepath.Join(stateRoot.Path, "example.com", normalized.Identifier("/news")),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if err = state.Configure(stateRoot, nil, tc.href); err != nil {
				t.Fatalf("state.Configure error: %v", err)
			} else {
				t.Logf("state configured from href: %q", tc.href)
				t.Logf("state configured with URI: %q", state.URI.Path)
			}
			if err := os.RemoveAll(stateRoot.Path); err != nil {
				t.Fatalf("error during testdata cleanup: %v", err)
			}
			if exists, err := state.Exists(ctx); err != nil || exists {
				t.Fatalf("state.Destroy failed (exists: %v, err: %q)\n", exists, err)
			}
			if err := state.Create(); err != nil {
				t.Fatalf("state.Create error: %v", err)
			} else {
				t.Logf("state created at %q", state.URI.Path)
			}
			if exists, err := state.Exists(ctx); err != nil || !exists {
				t.Fatalf("state.Create failed (exists: %v, err: %q)\n", exists, err)
			}
			if err := os.RemoveAll(stateRoot.Path); err != nil {
				t.Fatalf("error during testdata cleanup: %v", err)
			}
			if exists, err := state.Exists(ctx); err != nil || exists {
				t.Fatalf("state.Destroy failed, state still exists")
			} else {
				t.Logf("state destroyed at %q", state.URI.Path)
			}
		})
	}
}

func TestStatePut(t *testing.T) {
	var stateRoot *url.URL = &url.URL{Path: filepath.Join("testdata", t.Name()) + "/"}
	var state *State = &State{}
	var err error
	tests := []struct {
		name string
		href string
		data StateFile
		err  error
	}{
		{
			name: "example config",
			href: "https://example.com/feed/",
			data: &Example{Name: "Alice", Age: 30},
			err:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var file *File
			var ctx context.Context = context.Background()
			if err = state.Configure(stateRoot, nil, tc.href); tc.err == nil && err != nil {
				t.Fatalf("state.Configure error: %v", err)
			} else if tc.err != nil && errors.Is(err, tc.err) {
				t.Logf("expected error observed: %v", err)
				return // skip remaining tests on expected error
			}
			if err := os.RemoveAll(stateRoot.Path); err != nil {
				t.Fatalf("error during testdata cleanup: %v", err)
			}
			if err := state.Create(); err != nil {
				t.Fatalf("state.Create error: %v", err)
			} else {
				t.Logf("state created at %q", state.URI.Path)
			}

			file = &File{
				State: state,
				Data:  tc.data,
			}
			if err := file.Put(ctx); err != nil {
				t.Fatalf("file.Put error: %v", err)
			} else {
				t.Logf("file put at %q", file.URI.Path)
			}
		})
	}
}