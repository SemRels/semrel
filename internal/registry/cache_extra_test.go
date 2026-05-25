// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNormalizeCacheDir(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: filepath.Join(".semrel", registryCacheDirName)},
		{name: "append cache dir", input: filepath.Join("root", "cache"), want: filepath.Join("root", "cache", registryCacheDirName)},
		{name: "preserve cache dir", input: filepath.Join("root", registryCacheDirName), want: filepath.Join("root", registryCacheDirName)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCacheDir(tt.input); got != tt.want {
				t.Fatalf("normalizeCacheDir(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCacheHelpersReadWriteAndClear(t *testing.T) {
	client := newTestClient(t, DefaultBaseURL)
	metadata := []byte(`{"plugins":[{"name":"demo","versions":[{"version":"1.0.0","downloadUrl":"https://example.com/demo.exe","checksums":{"windows_amd64":"abc"}}]}]}`)

	written, err := client.writeCachedMetadata(metadata, ` "etag-demo" `)
	if err != nil {
		t.Fatalf("writeCachedMetadata() error = %v", err)
	}
	if written.Plugins[0].Name != "demo" {
		t.Fatalf("unexpected plugin name %q", written.Plugins[0].Name)
	}
	if !client.cacheFresh() {
		t.Fatal("expected cacheFresh() to be true after write")
	}

	cached, etag, err := client.readCachedMetadata()
	if err != nil {
		t.Fatalf("readCachedMetadata() error = %v", err)
	}
	if etag != `"etag-demo"` {
		t.Fatalf("etag = %q, want %q", etag, `"etag-demo"`)
	}
	if cached == nil || cached.Plugins[0].Name != "demo" {
		t.Fatalf("cached metadata = %#v", cached)
	}
	if client.semrelRootDir() != filepath.Dir(client.cacheDir) {
		t.Fatalf("semrelRootDir() = %q, want %q", client.semrelRootDir(), filepath.Dir(client.cacheDir))
	}
	if got := client.localPluginDir("demo", "1.0.0", "windows", "amd64"); !strings.Contains(got, filepath.Join("windows_amd64", "demo", "1.0.0")) {
		t.Fatalf("localPluginDir() = %q", got)
	}

	if _, err := client.writeCachedMetadata(metadata, ""); err != nil {
		t.Fatalf("writeCachedMetadata(remove etag) error = %v", err)
	}
	if _, err := os.Stat(client.metadataETagPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("metadata ETag file still exists, err = %v", err)
	}

	if err := client.ClearCache(); err != nil {
		t.Fatalf("ClearCache() error = %v", err)
	}
	if _, err := os.Stat(client.cacheDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache dir still exists, err = %v", err)
	}
}

func TestTouchMetadataCacheError(t *testing.T) {
	client := newTestClient(t, DefaultBaseURL)
	assertRegistryErrorCode(t, client.touchMetadataCache(), ErrCodeCacheError)
}

func TestRegistryErrorFormattingAndUnwrap(t *testing.T) {
	wrapped := errors.New("boom")
	err := &RegistryError{Code: ErrCodeCacheError, Message: "cache failed", Err: wrapped}
	if !strings.Contains(err.Error(), "registry cache_error: cache failed: boom") {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, wrapped) {
		t.Fatal("expected wrapped error to be discoverable")
	}

	var nilErr *RegistryError
	if got := nilErr.Error(); got != "<nil>" {
		t.Fatalf("nil Error() = %q, want <nil>", got)
	}
	if nilErr.Unwrap() != nil {
		t.Fatal("expected nil Unwrap() result")
	}
}

func TestMetadataLookupHelpers(t *testing.T) {
	registry := &PluginRegistry{Plugins: []PluginMeta{{
		Name: "demo",
		Versions: []PluginVersion{
			{Version: "1.0.0-beta.1", Prerelease: true, DownloadURL: "https://example.com/beta", Checksums: map[string]string{"linux_amd64": "beta"}},
			{Version: "1.0.0", DownloadURL: "https://example.com/stable", Checksums: map[string]string{"linux_amd64": "stable"}},
		},
	}}}

	pluginMeta, err := registry.FindPlugin("demo")
	if err != nil {
		t.Fatalf("FindPlugin() error = %v", err)
	}
	version, err := pluginMeta.FindVersion("")
	if err != nil {
		t.Fatalf("FindVersion() error = %v", err)
	}
	if version.Version != "1.0.0" {
		t.Fatalf("FindVersion(empty) = %q, want 1.0.0", version.Version)
	}
	checksum, err := version.ChecksumForPlatform("linux", "amd64")
	if err != nil {
		t.Fatalf("ChecksumForPlatform() error = %v", err)
	}
	if checksum != "stable" {
		t.Fatalf("checksum = %q, want stable", checksum)
	}

	if _, err := registry.FindPlugin("missing"); err == nil {
		t.Fatal("expected missing plugin error")
	}
	if _, err := pluginMeta.FindVersion("9.9.9"); err == nil {
		t.Fatal("expected missing version error")
	}
	if _, err := version.ChecksumForPlatform("windows", "amd64"); err == nil {
		t.Fatal("expected missing checksum error")
	}

	preOnly := &PluginMeta{Versions: []PluginVersion{{Version: "2.0.0-rc.1", Prerelease: true}, {Version: "2.0.0-rc.2", Prerelease: true}}}
	preVersion, err := preOnly.FindVersion("")
	if err != nil {
		t.Fatalf("FindVersion(prerelease only) error = %v", err)
	}
	if preVersion.Version != "2.0.0-rc.2" {
		t.Fatalf("FindVersion(prerelease only) = %q, want 2.0.0-rc.2", preVersion.Version)
	}
}

func TestDownloadWithRetryRetriesNetworkErrors(t *testing.T) {
	binary := []byte("retry-success")
	checksum := sha256Hex(binary)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(binary)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	target := filepath.Join(t.TempDir(), "plugin.exe")
	if err := client.downloadWithRetry(context.Background(), server.URL, target, checksum); err != nil {
		t.Fatalf("downloadWithRetry() error = %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != string(binary) {
		t.Fatalf("downloaded data = %q, want %q", string(data), string(binary))
	}
}

func TestDownloadHelpers(t *testing.T) {
	if got := downloadFileName("https://example.com/releases/demo.exe", "demo", "windows"); got != "demo.exe" {
		t.Fatalf("downloadFileName() = %q, want demo.exe", got)
	}
	if got := downloadFileName("://bad-url", "demo", "windows"); got != "demo.exe" {
		t.Fatalf("downloadFileName() invalid URL = %q, want demo.exe", got)
	}
	if got := downloadFileName("://bad-url", "demo", "linux"); got != "demo" {
		t.Fatalf("downloadFileName() invalid URL linux = %q, want demo", got)
	}

	if !isRetryableDownloadError(errors.New("plain error")) {
		t.Fatal("expected plain errors to be retryable")
	}
	if !isRetryableDownloadError(newRegistryError(ErrCodeNetworkError, "network", nil)) {
		t.Fatal("expected network registry error to be retryable")
	}
	if isRetryableDownloadError(newRegistryError(ErrCodeInvalidChecksum, "checksum", nil)) {
		t.Fatal("expected checksum error to be non-retryable")
	}

	assertRegistryErrorCode(t, newTestClient(t, DefaultBaseURL).ValidateChecksum(filepath.Join(t.TempDir(), "missing.exe"), strings.Repeat("0", 64)), ErrCodeCacheError)
}
