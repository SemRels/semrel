// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package monorepo provides workspace/package discovery and independent
// versioning support for monorepo repositories.
//
// semrel can operate in two monorepo modes:
//
//  1. Independent mode: each package/workspace gets its own semver version
//     and tag (e.g. "packages/api@v1.2.0", "packages/ui@v3.0.0").
//  2. Lockstep mode: all packages share the same version and are released
//     together (e.g. v1.2.0 tags all packages simultaneously).
//
// Package discovery finds workspaces by:
//   - Glob patterns in .semrel.yaml monorepo.packages
//   - Presence of go.mod, package.json, Cargo.toml, pyproject.toml, etc.
//   - Explicit package list in config
//
// See:
//   - https://github.com/SemRels/semrel/issues/40 (discovery)
//   - https://github.com/SemRels/semrel/issues/41 (independent versioning)
//   - https://github.com/SemRels/semrel/issues/42 (lockstep versioning)
package monorepo

import (
	"os"
	"path/filepath"
	"strings"
)

// VersioningMode controls how multiple packages are versioned.
type VersioningMode string

const (
	// ModeIndependent gives each package its own independent version.
	ModeIndependent VersioningMode = "independent"
	// ModeLockstep releases all packages at the same version simultaneously.
	ModeLockstep VersioningMode = "lockstep"
)

// Package represents a discovered workspace package in a monorepo.
type Package struct {
	// Name is the package name (e.g. "api", "packages/ui").
	Name string
	// Path is the absolute path to the package root directory.
	Path string
	// TagPrefix is the git tag prefix for this package (e.g. "api@v" or "v").
	TagPrefix string
	// ManifestFile is the package manifest detected (e.g. "go.mod", "package.json").
	ManifestFile string
}

// discoveryMarkers are files whose presence indicates a package root.
var discoveryMarkers = []string{
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"pom.xml",
	"build.gradle",
	"setup.py",
	".semrel.yaml",
}

// Discoverer discovers packages in a monorepo.
type Discoverer struct {
	// RootDir is the repository root directory.
	RootDir string
	// MaxDepth limits how deep the discovery walks (0 = unlimited).
	MaxDepth int
}

// NewDiscoverer creates a discoverer rooted at the given directory.
func NewDiscoverer(rootDir string) *Discoverer {
	return &Discoverer{RootDir: rootDir, MaxDepth: 5}
}

// DiscoverPackages walks the root directory tree and returns packages
// whose directories contain a known manifest file (go.mod, package.json, etc.).
// The root itself is excluded — only subdirectories are returned.
func (d *Discoverer) DiscoverPackages() ([]Package, error) {
	var packages []Package
	root, err := filepath.Abs(d.RootDir)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		// Skip the root itself
		if path == root {
			return nil
		}
		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		// Check depth
		rel, _ := filepath.Rel(root, path)
		depth := len(strings.Split(rel, string(os.PathSeparator)))
		if d.MaxDepth > 0 && depth > d.MaxDepth {
			return filepath.SkipDir
		}

		// Check for manifest files
		for _, marker := range discoveryMarkers {
			markerPath := filepath.Join(path, marker)
			if _, err := os.Stat(markerPath); err == nil {
				name := filepath.ToSlash(rel)
				pkg := Package{
					Name:         name,
					Path:         path,
					TagPrefix:    name + "@v",
					ManifestFile: marker,
				}
				packages = append(packages, pkg)
				// Don't descend into packages that have their own go.mod
				if marker == "go.mod" {
					return filepath.SkipDir
				}
				break
			}
		}
		return nil
	})

	return packages, err
}

// DiscoverFromPatterns returns packages matching the given glob patterns
// relative to RootDir.
func (d *Discoverer) DiscoverFromPatterns(patterns []string) ([]Package, error) {
	var packages []Package
	root, err := filepath.Abs(d.RootDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			if seen[match] {
				continue
			}
			seen[match] = true
			rel, _ := filepath.Rel(root, match)
			name := filepath.ToSlash(rel)
			pkg := Package{
				Name:      name,
				Path:      match,
				TagPrefix: name + "@v",
			}
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

// TagName returns the git tag for this package and version.
// E.g. Package{TagPrefix:"packages/api@v"}.TagName("1.2.0") = "packages/api@v1.2.0"
func (p Package) TagName(version string) string {
	return p.TagPrefix + version
}
