// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/internal/registry"
)

func TestNewLoaderUsesEnvironmentRegistryClient(t *testing.T) {
	binary := []byte("loader-binary")
	checksum := sha256.Sum256(binary)
	server := loaderTestServer(t, binary, fmt.Sprintf("%x", checksum[:]))
	defer server.Close()

	t.Setenv(registry.EnvRegistryURL, server.URL)
	t.Setenv(registry.EnvCacheDir, t.TempDir())

	loader := NewLoader()
	metadata, err := loader.FetchMetadata(context.Background())
	if err != nil {
		t.Fatalf("FetchMetadata() error = %v", err)
	}
	if len(metadata.Plugins) != 1 || metadata.Plugins[0].Name != "provider-github" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestLoaderPublicMethods(t *testing.T) {
	binary := []byte("plugin-binary")
	checksum := sha256.Sum256(binary)
	server := loaderTestServer(t, binary, fmt.Sprintf("%x", checksum[:]))
	defer server.Close()

	client, err := registry.NewRegistryClient(server.URL, t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	loader := NewLoaderWithRegistryClient(client)
	path, err := loader.ResolvePluginBinaryForPlatform(context.Background(), "provider-github", "1.0.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("ResolvePluginBinaryForPlatform() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != string(binary) {
		t.Fatalf("resolved binary = %q, want %q", string(data), string(binary))
	}
	if err := loader.ValidateChecksum(path, fmt.Sprintf("%x", checksum[:])); err != nil {
		t.Fatalf("ValidateChecksum() error = %v", err)
	}
	if err := loader.ValidateChecksum(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected checksum error")
	}

	currentClient, err := registry.NewRegistryClient(server.URL, t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistryClient() current platform error = %v", err)
	}
	currentLoader := NewLoaderWithRegistryClient(currentClient)
	if _, err := currentLoader.ResolvePluginBinary(context.Background(), "provider-github", "1.0.0"); err != nil {
		t.Fatalf("ResolvePluginBinary() error = %v", err)
	}
}

func TestLoaderErrorPaths(t *testing.T) {
	loader := &Loader{initErr: errors.New("init failed")}
	if _, err := loader.FetchMetadata(context.Background()); err == nil || !strings.Contains(err.Error(), "init failed") {
		t.Fatalf("FetchMetadata() error = %v, want init failure", err)
	}

	loader = &Loader{}
	if _, err := loader.ResolvePluginBinaryForPlatform(context.Background(), "demo", "1.0.0", runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "registry client is not configured") {
		t.Fatalf("ResolvePluginBinaryForPlatform() error = %v", err)
	}
	if err := loader.ValidateChecksum("missing", "deadbeef"); err == nil || !strings.Contains(err.Error(), "registry client is not configured") {
		t.Fatalf("ValidateChecksum() error = %v", err)
	}

	if _, err := NewLoaderWithRegistryClient(nil).FetchMetadata(context.Background()); err == nil {
		t.Fatal("expected nil registry client error")
	}
}

func TestLoadPluginsNotImplemented(t *testing.T) {
	_, err := NewLoaderWithRegistryClient(nil).LoadPlugins("plugins")
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("LoadPlugins() error = %v", err)
	}
}

func loaderTestServer(t *testing.T, binary []byte, checksum string) *httptest.Server {
	t.Helper()

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugins.json":
			_, _ = w.Write([]byte(loaderMetadataJSON(serverURL, checksum)))
		case "/downloads/provider-github.exe":
			_, _ = w.Write(binary)
		case "/downloads/provider-github":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	return server
}

func loaderMetadataJSON(serverURL, checksum string) string {
	return fmt.Sprintf(`{"plugins":[{"name":"provider-github","versions":[{"version":"1.0.0","downloadUrl":"%s/downloads/provider-github.exe","checksums":{"windows_amd64":"%s","linux_amd64":"%s","darwin_amd64":"%s","darwin_arm64":"%s","windows_arm64":"%s"}}]}]}`,
		serverURL,
		checksum,
		checksum,
		checksum,
		checksum,
		checksum,
	)
}
