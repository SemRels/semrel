// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package monorepo

import (
	"os"
	"path/filepath"
	"testing"
)

// helpers

func makeDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	makeDir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// tests

func TestDiscoverPackages_Empty(t *testing.T) {
	dir := t.TempDir()
	d := NewDiscoverer(dir)
	pkgs, err := d.DiscoverPackages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Errorf("expected no packages in empty dir, got %d", len(pkgs))
	}
}

func TestDiscoverPackages_GoMod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "api", "go.mod"), "module api\n\ngo 1.21\n")
	writeFile(t, filepath.Join(root, "ui", "package.json"), `{"name":"ui"}`)

	d := NewDiscoverer(root)
	pkgs, err := d.DiscoverPackages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %v", len(pkgs), pkgs)
	}

	names := make(map[string]bool)
	for _, p := range pkgs {
		names[p.Name] = true
	}
	if !names["api"] {
		t.Error("expected api package")
	}
	if !names["ui"] {
		t.Error("expected ui package")
	}
}

func TestDiscoverPackages_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".hidden", "go.mod"), "module hidden\n\ngo 1.21\n")
	writeFile(t, filepath.Join(root, "visible", "go.mod"), "module visible\n\ngo 1.21\n")

	d := NewDiscoverer(root)
	pkgs, err := d.DiscoverPackages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range pkgs {
		if p.Name == ".hidden" {
			t.Error("hidden dir should be skipped")
		}
	}
	found := false
	for _, p := range pkgs {
		if p.Name == "visible" {
			found = true
		}
	}
	if !found {
		t.Error("expected visible package")
	}
}

func TestDiscoverPackages_RootExcluded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module root\n\ngo 1.21\n")
	writeFile(t, filepath.Join(root, "sub", "go.mod"), "module sub\n\ngo 1.21\n")

	d := NewDiscoverer(root)
	pkgs, err := d.DiscoverPackages()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range pkgs {
		if p.Path == root {
			t.Error("root dir should not appear as a discovered package")
		}
	}
	if len(pkgs) != 1 || pkgs[0].Name != "sub" {
		t.Errorf("expected only sub package, got %v", pkgs)
	}
}

func TestPackage_TagName(t *testing.T) {
	p := Package{Name: "packages/api", TagPrefix: "packages/api@v"}
	if got := p.TagName("1.2.0"); got != "packages/api@v1.2.0" {
		t.Errorf("expected packages/api@v1.2.0, got %q", got)
	}
}

func TestDiscoverFromPatterns(t *testing.T) {
	root := t.TempDir()
	makeDir(t, filepath.Join(root, "packages", "core"))
	makeDir(t, filepath.Join(root, "packages", "utils"))
	makeDir(t, filepath.Join(root, "apps", "web"))

	d := NewDiscoverer(root)
	pkgs, err := d.DiscoverFromPatterns([]string{"packages/*", "apps/*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 3 {
		t.Errorf("expected 3 packages, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverFromPatterns_Deduplication(t *testing.T) {
	root := t.TempDir()
	makeDir(t, filepath.Join(root, "pkg", "core"))

	d := NewDiscoverer(root)
	// Same directory matches two patterns — should deduplicate
	pkgs, err := d.DiscoverFromPatterns([]string{"pkg/core", "pkg/core"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Errorf("expected 1 deduplicated package, got %d", len(pkgs))
	}
}

func TestVersioningMode_Constants(t *testing.T) {
	if ModeIndependent != "independent" {
		t.Errorf("expected independent, got %q", ModeIndependent)
	}
	if ModeLockstep != "lockstep" {
		t.Errorf("expected lockstep, got %q", ModeLockstep)
	}
}
