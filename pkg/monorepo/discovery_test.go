// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package monorepo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoSemantics/semrel/pkg/monorepo"
)

func TestDiscoverPackages_Basic(t *testing.T) {
	root := t.TempDir()
	// Create two sub-packages
	pkgA := filepath.Join(root, "packages", "core")
	pkgB := filepath.Join(root, "packages", "api")
	os.MkdirAll(pkgA, 0o755)
	os.MkdirAll(pkgB, 0o755)
	os.WriteFile(filepath.Join(pkgA, "go.mod"), []byte("module core"), 0o644)
	os.WriteFile(filepath.Join(pkgB, "package.json"), []byte("{}"), 0o644)

	d := monorepo.NewDiscoverer(root)
	pkgs, err := d.DiscoverPackages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
}

func TestDiscoverPackages_SkipsHidden(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".hidden", "pkg")
	os.MkdirAll(hidden, 0o755)
	os.WriteFile(filepath.Join(hidden, "go.mod"), []byte("module hidden"), 0o644)

	d := monorepo.NewDiscoverer(root)
	pkgs, err := d.DiscoverPackages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("expected 0 packages (hidden skipped), got %d", len(pkgs))
	}
}

func TestDiscoverFromPatterns_Basic(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "packages", "core"), 0o755)
	os.MkdirAll(filepath.Join(root, "packages", "api"), 0o755)

	d := monorepo.NewDiscoverer(root)
	pkgs, err := d.DiscoverFromPatterns([]string{"packages/*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}
}

func TestPackage_TagName(t *testing.T) {
	p := monorepo.Package{
		Name:      "packages/api",
		Path:      "/repo/packages/api",
		TagPrefix: "packages/api@v",
	}
	got := p.TagName("1.2.3")
	want := "packages/api@v1.2.3"
	if got != want {
		t.Errorf("TagName: got %q, want %q", got, want)
	}
}

func TestDiscoverFromPatterns_Dedup(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "packages", "core"), 0o755)

	d := monorepo.NewDiscoverer(root)
	// Same path appears from two patterns
	pkgs, err := d.DiscoverFromPatterns([]string{"packages/*", "packages/core"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package (dedup), got %d", len(pkgs))
	}
}
