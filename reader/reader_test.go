package reader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	tests := []struct {
		name       string
		raw        string
		wantScheme string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "bare relative path",
			raw:        "subscriptions.opml",
			wantScheme: "file",
			wantPath:   filepath.Join(pwd, "subscriptions.opml"),
		},
		{
			name:       "dot relative path",
			raw:        "./feeds/subscriptions.opml",
			wantScheme: "file",
			wantPath:   filepath.Join(pwd, "feeds", "subscriptions.opml"),
		},
		{
			name:       "absolute path",
			raw:        "/tmp/subscriptions.opml",
			wantScheme: "file",
			wantPath:   "/tmp/subscriptions.opml",
		},
		{
			name:       "file URL",
			raw:        "file:///tmp/subscriptions.opml",
			wantScheme: "file",
			wantPath:   "/tmp/subscriptions.opml",
		},
		{
			name:       "file URL with percent-encoded space (Swift absoluteString)",
			raw:        "file:///tmp/my%20subscriptions.opml",
			wantScheme: "file",
			wantPath:   "/tmp/my subscriptions.opml",
		},
		{
			name:       "path with space",
			raw:        "/tmp/my subscriptions.opml",
			wantScheme: "file",
			wantPath:   "/tmp/my subscriptions.opml",
		},
		{
			name:       "path with percent",
			raw:        "/tmp/100%done/feed.opml",
			wantScheme: "file",
			wantPath:   "/tmp/100%done/feed.opml",
		},
		{
			name:       "path with hash",
			raw:        "/tmp/my#feed.opml",
			wantScheme: "file",
			wantPath:   "/tmp/my#feed.opml",
		},
		{
			name:       "path with question mark",
			raw:        "/tmp/my?feed.opml",
			wantScheme: "file",
			wantPath:   "/tmp/my?feed.opml",
		},
		{
			name:       "relative path with colon",
			raw:        "notes:2024.opml",
			wantScheme: "file",
			wantPath:   filepath.Join(pwd, "notes:2024.opml"),
		},
		{
			name:       "http URL",
			raw:        "http://example.com/subscriptions.opml",
			wantScheme: "http",
			wantPath:   "/subscriptions.opml",
		},
		{
			name:       "https URL",
			raw:        "https://example.com/subscriptions.opml",
			wantScheme: "https",
			wantPath:   "/subscriptions.opml",
		},
		{
			name:    "empty string",
			raw:     "",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u, err := Normalize(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Normalize(%q) expected error; got %q", test.raw, u.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize(%q) error = %v", test.raw, err)
			}
			if u.URL().Scheme != test.wantScheme {
				t.Errorf("Normalize(%q) scheme = %q; want %q", test.raw, u.URL().Scheme, test.wantScheme)
			}
			if u.URL().Path != test.wantPath {
				t.Errorf("Normalize(%q) path = %q; want %q", test.raw, u.URL().Path, test.wantPath)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	var ctx = context.Background()
	var contents = []byte("<opml version=\"2.0\"><head/><body/></opml>")
	path := filepath.Join(t.TempDir(), "subscriptions.opml")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	u, err := Normalize(path)
	if err != nil {
		t.Fatalf("Normalize(%q) error = %v", path, err)
	}
	body, err := Read(ctx, Client(DefaultConfig), u, DefaultConfig)
	if err != nil {
		t.Fatalf("Read(%q) error = %v", u.String(), err)
	}
	if string(body) != string(contents) {
		t.Errorf("Read(%q) = %q; want %q", u.String(), body, contents)
	}
}

// TestReadFileMetacharacters exercises file reads through paths containing URL
// metacharacters that are legal in filenames (space, %, #, ?).
func TestReadFileMetacharacters(t *testing.T) {
	var ctx = context.Background()
	var contents = []byte("<rss version=\"2.0\"></rss>")
	for _, name := range []string{
		"my generated feed.xml",
		"100%done.xml",
		"my#feed.xml",
		"my?feed.xml",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, contents, 0o644); err != nil {
				t.Fatalf("os.WriteFile() error = %v", err)
			}
			u, err := Normalize(path)
			if err != nil {
				t.Fatalf("Normalize(%q) error = %v", path, err)
			}
			body, err := Read(ctx, Client(DefaultConfig), u, DefaultConfig)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", u.String(), err)
			}
			if string(body) != string(contents) {
				t.Errorf("Read(%q) = %q; want %q", u.String(), body, contents)
			}
		})
	}
}

// TestReadRejectsCrossSchemeRedirect verifies a remote server cannot redirect a
// read into the file transport to exfiltrate a local file.
func TestReadRejectsCrossSchemeRedirect(t *testing.T) {
	var ctx = context.Background()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file://"+secret, http.StatusFound)
	}))
	defer server.Close()
	u, err := Normalize(server.URL + "/feed.xml")
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	body, err := Read(ctx, Client(DefaultConfig), u, DefaultConfig)
	if err == nil {
		t.Fatalf("Read(%q) followed cross-scheme redirect and returned %q; want error", u.String(), body)
	}
	if !errors.Is(err, ErrRead) {
		t.Errorf("Read(%q) error = %v; want ErrRead", u.String(), err)
	}
}

func TestReadFileNotFound(t *testing.T) {
	var ctx = context.Background()
	u, err := Normalize(filepath.Join(t.TempDir(), "missing.opml"))
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if _, err = Read(ctx, Client(DefaultConfig), u, DefaultConfig); err == nil {
		t.Fatalf("Read(%q) expected error; got nil", u.String())
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("Read(%q) error = %v; want ErrNotFound", u.String(), err)
	}
}

func TestReadMaxContentLength(t *testing.T) {
	var ctx = context.Background()
	var config = Config{Timeout: DefaultConfig.Timeout, MaxContentLength: 8}
	path := filepath.Join(t.TempDir(), "too-big.xml")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 16)), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	u, err := Normalize(path)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if _, err = Read(ctx, Client(config), u, config); err == nil {
		t.Fatalf("Read(%q) expected error; got nil", u.String())
	} else if !errors.Is(err, ErrContentLength) {
		t.Errorf("Read(%q) error = %v; want ErrContentLength", u.String(), err)
	}
}

func TestReadHTTP(t *testing.T) {
	var ctx = context.Background()
	var contents = "<opml version=\"2.0\"><head/><body/></opml>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-opml+xml")
		_, _ = w.Write([]byte(contents))
	}))
	defer server.Close()
	u, err := Normalize(server.URL + "/subscriptions.opml")
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	body, err := Read(ctx, Client(DefaultConfig), u, DefaultConfig)
	if err != nil {
		t.Fatalf("Read(%q) error = %v", u.String(), err)
	}
	if string(body) != contents {
		t.Errorf("Read(%q) = %q; want %q", u.String(), body, contents)
	}
}

func TestReadHTTPError(t *testing.T) {
	var ctx = context.Background()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	u, err := Normalize(server.URL + "/missing.opml")
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if _, err = Read(ctx, Client(DefaultConfig), u, DefaultConfig); err == nil {
		t.Fatalf("Read(%q) expected error; got nil", u.String())
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("Read(%q) error = %v; want ErrNotFound", u.String(), err)
	}
}

// TestSchemeDispatch verifies one client serves both file:// and http:// reads.
func TestSchemeDispatch(t *testing.T) {
	var ctx = context.Background()
	var client = Client(DefaultConfig)

	// file://
	path := filepath.Join(t.TempDir(), "feed.xml")
	if err := os.WriteFile(path, []byte("local"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	fileURL, err := Normalize(path)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if body, err := Read(ctx, client, fileURL, DefaultConfig); err != nil {
		t.Fatalf("Read(%q) error = %v", fileURL.String(), err)
	} else if string(body) != "local" {
		t.Errorf("Read(%q) = %q; want %q", fileURL.String(), body, "local")
	}

	// http://
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("remote"))
	}))
	defer server.Close()
	httpURL, err := Normalize(server.URL)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if body, err := Read(ctx, client, httpURL, DefaultConfig); err != nil {
		t.Fatalf("Read(%q) error = %v", httpURL.String(), err)
	} else if string(body) != "remote" {
		t.Errorf("Read(%q) = %q; want %q", httpURL.String(), body, "remote")
	}
}
