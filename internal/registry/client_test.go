// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The go-semrel Authors

package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchMetadataUsesCacheAndETag(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	var serverURL string
	const etag = `"registry-v1"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "go-semrel/") {
			t.Fatalf("expected go-semrel user agent, got %q", got)
		}
		if requestCount.Load() > 1 {
			if got := r.Header.Get("If-None-Match"); got != etag {
				t.Fatalf("expected If-None-Match %q, got %q", etag, got)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(testMetadataJSON(serverURL, checksumPlaceholder)))
	}))
	serverURL = server.URL
	defer server.Close()

	client := newTestClient(t, server.URL)
	metadata, err := client.FetchMetadata(context.Background())
	if err != nil {
		t.Fatalf("FetchMetadata() error = %v", err)
	}
	if len(metadata.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(metadata.Plugins))
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected 1 request after initial fetch, got %d", got)
	}

	metadata, err = client.FetchMetadata(context.Background())
	if err != nil {
		t.Fatalf("FetchMetadata() cached error = %v", err)
	}
	if len(metadata.Plugins) != 1 {
		t.Fatalf("expected cached metadata, got %d plugins", len(metadata.Plugins))
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected cache hit without HTTP request, got %d requests", got)
	}

	makeCacheStale(t, client)
	metadata, err = client.FetchMetadata(context.Background())
	if err != nil {
		t.Fatalf("FetchMetadata() stale error = %v", err)
	}
	if len(metadata.Plugins) != 1 {
		t.Fatalf("expected cached metadata after 304, got %d plugins", len(metadata.Plugins))
	}
	if got := requestCount.Load(); got != 2 {
		t.Fatalf("expected 2 requests after revalidation, got %d", got)
	}
}

func TestFetchMetadataFallsBackToStaleCacheOnNetworkError(t *testing.T) {
	t.Parallel()

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testMetadataJSON(serverURL, checksumPlaceholder)))
	}))
	serverURL = server.URL

	client := newTestClient(t, server.URL)
	if _, err := client.FetchMetadata(context.Background()); err != nil {
		t.Fatalf("initial FetchMetadata() error = %v", err)
	}
	server.Close()

	makeCacheStale(t, client)
	metadata, err := client.FetchMetadata(context.Background())
	if err != nil {
		t.Fatalf("FetchMetadata() fallback error = %v", err)
	}
	if len(metadata.Plugins) != 1 {
		t.Fatalf("expected stale cached metadata, got %d plugins", len(metadata.Plugins))
	}
}

func TestFetchMetadataErrors(t *testing.T) {
	t.Parallel()

	t.Run("offline without cache", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t, "http://127.0.0.1:1")
		_, err := client.FetchMetadata(context.Background())
		assertRegistryErrorCode(t, err, ErrCodeNetworkError)
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"plugins":`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL)
		_, err := client.FetchMetadata(context.Background())
		assertRegistryErrorCode(t, err, ErrCodeInvalidMetadata)
	})
}

func TestDownloadPluginUsesLocalDiscovery(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "http://127.0.0.1:1")
	localDir := client.localPluginDir("provider-github", "1.0.0", "linux", "amd64")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	existingPath := filepath.Join(localDir, "provider-github")
	if err := os.WriteFile(existingPath, []byte("local-binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := client.DownloadPlugin(context.Background(), "provider-github", "1.0.0", "linux", "amd64")
	if err != nil {
		t.Fatalf("DownloadPlugin() error = %v", err)
	}
	if got != existingPath {
		t.Fatalf("expected local path %q, got %q", existingPath, got)
	}
}

func TestDownloadPluginDownloadsAndValidatesChecksum(t *testing.T) {
	t.Parallel()

	binary := []byte("test-plugin-binary")
	checksum := sha256Hex(binary)
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugins.json":
			_, _ = w.Write([]byte(testMetadataJSON(serverURL, checksum)))
		case "/downloads/provider-github.exe":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	defer server.Close()

	client := newTestClient(t, server.URL)
	path, err := client.DownloadPlugin(context.Background(), "provider-github", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("DownloadPlugin() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != string(binary) {
		t.Fatalf("unexpected download contents: got %q want %q", string(data), string(binary))
	}
	if err := client.ValidateChecksum(path, checksum); err != nil {
		t.Fatalf("ValidateChecksum() error = %v", err)
	}
}

func TestDownloadPluginErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing download", func(t *testing.T) {
		t.Parallel()
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/plugins.json":
				_, _ = w.Write([]byte(testMetadataJSON(serverURL, checksumPlaceholder)))
			default:
				http.NotFound(w, r)
			}
		}))
		serverURL = server.URL
		defer server.Close()

		client := newTestClient(t, server.URL)
		_, err := client.DownloadPlugin(context.Background(), "provider-github", "1.0.0", "windows", "amd64")
		assertRegistryErrorCode(t, err, ErrCodeNotFound)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		t.Parallel()
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/plugins.json":
				_, _ = w.Write([]byte(testMetadataJSON(serverURL, strings.Repeat("0", 64))))
			case "/downloads/provider-github.exe":
				_, _ = w.Write([]byte("bad-binary"))
			default:
				http.NotFound(w, r)
			}
		}))
		serverURL = server.URL
		defer server.Close()

		client := newTestClient(t, server.URL)
		_, err := client.DownloadPlugin(context.Background(), "provider-github", "1.0.0", "windows", "amd64")
		assertRegistryErrorCode(t, err, ErrCodeInvalidChecksum)
	})
}

func TestValidateChecksum(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, DefaultBaseURL)
	path := filepath.Join(t.TempDir(), "plugin.exe")
	contents := []byte("checksum-me")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := client.ValidateChecksum(path, sha256Hex(contents)); err != nil {
		t.Fatalf("ValidateChecksum() valid error = %v", err)
	}
	assertRegistryErrorCode(t, client.ValidateChecksum(path, strings.Repeat("f", 64)), ErrCodeInvalidChecksum)
}

func newTestClient(t *testing.T, baseURL string) *RegistryClient {
	t.Helper()
	client, err := NewRegistryClient(baseURL, t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}
	client.cacheTTL = time.Hour
	return client
}

func makeCacheStale(t *testing.T, client *RegistryClient) {
	t.Helper()
	staleTime := time.Now().Add(-(client.cacheTTL + time.Minute))
	if err := os.Chtimes(client.metadataCachePath(), staleTime, staleTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
}

func assertRegistryErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var registryErr *RegistryError
	if !errors.As(err, &registryErr) {
		t.Fatalf("expected RegistryError, got %T: %v", err, err)
	}
	if registryErr.Code != want {
		t.Fatalf("expected error code %q, got %q", want, registryErr.Code)
	}
}

const checksumPlaceholder = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func testMetadataJSON(serverURL, checksum string) string {
	return fmt.Sprintf(`{"plugins":[{"name":"provider-github","description":"GitHub provider","author":"GoSemantics","license":"Apache-2.0","category":"provider","versions":[{"version":"1.0.0","releaseDate":"2026-05-24T12:00:00Z","downloadUrl":"%s/downloads/provider-github.exe","checksums":{"windows_amd64":"%s","linux_amd64":"%s"}}]}]}`,
		serverURL,
		checksum,
		checksum,
	)
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:])
}
