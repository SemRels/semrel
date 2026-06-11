// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SemRels/semrel/internal/registry"
)

func TestReadLockFile_NotExist(t *testing.T) {
	withWorkingDir(t, t.TempDir())
	lf, err := ReadLockFile()
	if err != nil {
		t.Fatalf("ReadLockFile() error = %v", err)
	}
	if len(lf.Plugins) != 0 {
		t.Fatalf("expected empty plugins, got %d", len(lf.Plugins))
	}
}

func TestPluginLockFile_UpsertAndFind(t *testing.T) {
	lf := &PluginLockFile{}

	entry := PluginLockEntry{
		BinaryName: "semrel-plugin-github",
		Ref:        "@SemRels/github",
		Version:    "1.2.0",
		Checksums:  map[string]string{"linux_amd64": "abc123"},
	}
	lf.Upsert(entry)
	if len(lf.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(lf.Plugins))
	}

	// Upsert with same BinaryName should replace.
	updated := entry
	updated.Version = "1.3.0"
	lf.Upsert(updated)
	if len(lf.Plugins) != 1 {
		t.Fatalf("expected 1 plugin after upsert, got %d", len(lf.Plugins))
	}
	if lf.Plugins[0].Version != "1.3.0" {
		t.Fatalf("expected updated version 1.3.0, got %q", lf.Plugins[0].Version)
	}

	// FindByBinaryName.
	found := lf.FindByBinaryName("semrel-plugin-github")
	if found == nil || found.Version != "1.3.0" {
		t.Fatalf("FindByBinaryName() = %v", found)
	}
	if lf.FindByBinaryName("semrel-plugin-missing") != nil {
		t.Fatal("expected nil for unknown binary")
	}
}

func TestPluginLockFile_WriteAndRead(t *testing.T) {
	withWorkingDir(t, t.TempDir())

	lf := &PluginLockFile{}
	lf.Upsert(PluginLockEntry{
		BinaryName: "semrel-plugin-github",
		Ref:        "@SemRels/github",
		Version:    "1.0.0",
		Checksums:  map[string]string{"linux_amd64": "deadbeef"},
	})
	if err := lf.Write(); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// File should exist.
	if _, err := os.Stat(LockFileName); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}

	// Round-trip read.
	lf2, err := ReadLockFile()
	if err != nil {
		t.Fatalf("ReadLockFile() error = %v", err)
	}
	if len(lf2.Plugins) != 1 {
		t.Fatalf("expected 1 plugin after round-trip, got %d", len(lf2.Plugins))
	}
	if lf2.Plugins[0].Ref != "@SemRels/github" {
		t.Fatalf("ref = %q", lf2.Plugins[0].Ref)
	}
	if lf2.Plugins[0].Checksums["linux_amd64"] != "deadbeef" {
		t.Fatalf("checksum = %q", lf2.Plugins[0].Checksums["linux_amd64"])
	}
}

func TestRunPluginInstallWritesLockFile(t *testing.T) {
	withColorsDisabled(t)
	binary := []byte("plugin-binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(binary))
	server := newCLIRegistryServer(t, binary, checksum)
	defer server.Close()

	t.Setenv(registry.EnvRegistryURL, server.URL)
	repoDir := t.TempDir()
	t.Setenv(registry.EnvCacheDir, filepath.Join(repoDir, ".semrel", "registry-cache"))
	withWorkingDir(t, repoDir)

	// Install to the default project-local dir (no overrideDir).
	_, _, err := captureReleaseOutput(func() error {
		return runPluginInstall(context.Background(), "provider-github@1.0.0", "")
	})
	if err != nil {
		t.Fatalf("runPluginInstall() error = %v", err)
	}

	// .semrel.lock should have been created.
	lf, err := ReadLockFile()
	if err != nil {
		t.Fatalf("ReadLockFile() error = %v", err)
	}
	if len(lf.Plugins) == 0 {
		t.Fatal("expected lock file to contain at least one entry")
	}
	entry := lf.FindByBinaryName("semrel-plugin-github")
	if entry == nil {
		t.Fatalf("lock entry for semrel-plugin-github not found; entries = %v", lf.Plugins)
	}
	if entry.Version != "1.0.0" {
		t.Fatalf("locked version = %q, want 1.0.0", entry.Version)
	}
}

func TestRunPluginInstallDoesNotWriteLockWhenOverrideDirSet(t *testing.T) {
	withColorsDisabled(t)
	binary := []byte("plugin-binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(binary))
	server := newCLIRegistryServer(t, binary, checksum)
	defer server.Close()

	t.Setenv(registry.EnvRegistryURL, server.URL)
	repoDir := t.TempDir()
	t.Setenv(registry.EnvCacheDir, filepath.Join(repoDir, ".semrel", "registry-cache"))
	withWorkingDir(t, repoDir)

	installDir := t.TempDir()
	_, _, err := captureReleaseOutput(func() error {
		return runPluginInstall(context.Background(), "provider-github@1.0.0", installDir)
	})
	if err != nil {
		t.Fatalf("runPluginInstall() error = %v", err)
	}

	// No lock file should have been created when overrideDir is set.
	if _, statErr := os.Stat(LockFileName); statErr == nil {
		t.Fatal("expected no lock file when overrideDir is set")
	}
}

func TestRunPluginRestore(t *testing.T) {
	withColorsDisabled(t)
	binary := []byte("plugin-binary")
	checksum := fmt.Sprintf("%x", sha256.Sum256(binary))

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
	defer server.Close()

	t.Setenv(registry.EnvRegistryURL, serverURL)
	repoDir := t.TempDir()
	t.Setenv(registry.EnvCacheDir, filepath.Join(repoDir, ".semrel", "registry-cache"))
	withWorkingDir(t, repoDir)

	// Write a lock file manually.
	lf := &PluginLockFile{}
	lf.Upsert(PluginLockEntry{
		BinaryName: "semrel-plugin-github",
		Ref:        "provider-github",
		Version:    "1.0.0",
		Checksums: map[string]string{
			"linux_amd64":   checksum,
			"linux_arm64":   checksum,
			"darwin_amd64":  checksum,
			"darwin_arm64":  checksum,
			"windows_amd64": checksum,
			"windows_arm64": checksum,
		},
	})
	if err := lf.Write(); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	stdout, _, err := captureReleaseOutput(func() error {
		return runPluginRestore(context.Background())
	})
	if err != nil {
		t.Fatalf("runPluginRestore() error = %v", err)
	}
	if !strings.Contains(stdout, "restored") && !strings.Contains(stdout, "already installed") {
		t.Fatalf("stdout = %q — expected restore/already-installed message", stdout)
	}

	// Binary should be installed.
	dest := filepath.Join(repoDir, ".semrel", "plugins", "semrel-plugin-github")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("installed binary not found at %s: %v", dest, err)
	}
}

func TestRunPluginRestoreEmpty(t *testing.T) {
	withWorkingDir(t, t.TempDir())
	stdout, _, err := captureReleaseOutput(func() error {
		return runPluginRestore(context.Background())
	})
	if err != nil {
		t.Fatalf("runPluginRestore() error = %v", err)
	}
	if !strings.Contains(stdout, "nothing to restore") {
		t.Fatalf("stdout = %q", stdout)
	}
}
