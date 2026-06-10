// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SemRels/semrel/internal/registry"
)

func TestRunPluginListAndSearch(t *testing.T) {
	withColorsDisabled(t)
	binary := []byte("plugin-binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(binary))
	server := newCLIRegistryServer(t, binary, checksum)
	defer server.Close()

	t.Setenv(registry.EnvRegistryURL, server.URL)
	t.Setenv(registry.EnvCacheDir, t.TempDir())

	stdout, stderr, err := captureReleaseOutput(func() error {
		return runPluginList(context.Background(), false)
	})
	if err != nil {
		t.Fatalf("runPluginList() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "NAME") || !strings.Contains(stdout, "provider-github") || !strings.Contains(stdout, "1.1.0") {
		t.Fatalf("stdout = %q", stdout)
	}

	stdout, stderr, err = captureReleaseOutput(func() error {
		return runPluginSearch(context.Background(), "hooks")
	})
	if err != nil {
		t.Fatalf("runPluginSearch() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "provider-github") {
		t.Fatalf("stdout = %q", stdout)
	}

	stdout, stderr, err = captureReleaseOutput(func() error {
		return runPluginSearch(context.Background(), "missing-plugin")
	})
	if err != nil {
		t.Fatalf("runPluginSearch() error = %v", err)
	}
	if stdout == "" {
		t.Fatal("expected search table output")
	}
	if !strings.Contains(stderr, `no plugins matching "missing-plugin"`) {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunPluginInstallAndLint(t *testing.T) {
	withColorsDisabled(t)
	binary := []byte("plugin-binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(binary))
	server := newCLIRegistryServer(t, binary, checksum)
	defer server.Close()

	t.Setenv(registry.EnvRegistryURL, server.URL)
	t.Setenv(registry.EnvCacheDir, t.TempDir())

	installDir := t.TempDir()
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runPluginInstall(context.Background(), "provider-github@1.0.0", installDir)
	})
	if err != nil {
		t.Fatalf("runPluginInstall() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Installing provider-github@1.0.0") || !strings.Contains(stdout, "Installed provider-github") {
		t.Fatalf("stdout = %q", stdout)
	}

	installed := filepath.Join(installDir, pluginBinaryName("provider-github"))
	if runtime.GOOS == "windows" {
		installed += ".exe"
	}
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("ReadFile(installed) error = %v", err)
	}
	if string(data) != string(binary) {
		t.Fatalf("installed binary = %q, want %q", string(data), string(binary))
	}

	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v1.0.0")
	commitReleaseFile(t, repoDir, "bad.txt", "bad\n", "not a conventional commit")
	if err := os.WriteFile(filepath.Join(repoDir, ".semrel.toml"), []byte("[[branches]]\nname = \"main\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.semrel.toml) error = %v", err)
	}
	withWorkingDir(t, repoDir)

	stdout, stderr, err = captureReleaseOutput(func() error {
		return runLint(context.Background(), "", "json")
	})
	if err == nil {
		t.Fatal("expected lint failure")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for json output", stderr)
	}
	var summary LintSummary
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &summary); jsonErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", jsonErr)
	}
	if summary.Valid || len(summary.Invalid) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func newCLIRegistryServer(t *testing.T, binary []byte, checksum string) *httptest.Server {
	t.Helper()

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugins.json":
			_, _ = w.Write([]byte(cliRegistryMetadataJSON(serverURL, checksum)))
		case "/downloads/provider-github.exe":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	return server
}

func cliRegistryMetadataJSON(serverURL, checksum string) string {
	return fmt.Sprintf(`{"plugins":[{"name":"provider-github","description":"GitHub release hooks","category":"hooks","tags":["github","hooks"],"versions":[{"version":"1.0.0","downloadUrl":"%s/downloads/provider-github.exe","checksums":{"windows_amd64":"%s","windows_arm64":"%s","linux_amd64":"%s","darwin_amd64":"%s","darwin_arm64":"%s"}},{"version":"1.1.0","downloadUrl":"%s/downloads/provider-github.exe","checksums":{"windows_amd64":"%s","windows_arm64":"%s","linux_amd64":"%s","darwin_amd64":"%s","darwin_arm64":"%s"}}]}]}`,
		serverURL,
		checksum,
		checksum,
		checksum,
		checksum,
		checksum,
		serverURL,
		checksum,
		checksum,
		checksum,
		checksum,
		checksum,
	)
}
