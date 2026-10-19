// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"path/filepath"
	"strings"
)

// packageInfo identifies the workspace package a release run belongs to.
type packageInfo struct {
	// Name is the package name as configured (usually its path in the repo).
	Name string
	// Path is the absolute package root directory.
	Path string
}

type packageInfoKey struct{}

// withPackage marks ctx as belonging to a workspace package. The path is made
// absolute now, so it stays valid after the runner changes directory.
func withPackage(ctx context.Context, name, path string) context.Context {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return context.WithValue(ctx, packageInfoKey{}, &packageInfo{Name: name, Path: path})
}

// packageFrom returns the workspace package stored in ctx, or nil outside
// workspace mode.
func packageFrom(ctx context.Context) *packageInfo {
	p, _ := ctx.Value(packageInfoKey{}).(*packageInfo)
	return p
}

// changelogPath returns the absolute CHANGELOG.md path for a release run: the
// package root in workspace mode, the current directory otherwise. Absolute,
// because git commands run from the repository root, not from the cwd.
func changelogPath(ctx context.Context) string {
	if p := packageFrom(ctx); p != nil {
		return filepath.Join(p.Path, "CHANGELOG.md")
	}
	abs, err := filepath.Abs("CHANGELOG.md")
	if err != nil {
		return "CHANGELOG.md"
	}
	return abs
}

// changelogCommitMessage builds the changelog commit message, naming the
// package in workspace mode unless the tag already carries its name.
func changelogCommitMessage(ctx context.Context, tag string) string {
	if p := packageFrom(ctx); p != nil && !strings.Contains(tag, p.Name) {
		return "chore(changelog): update " + p.Name + " for " + tag + " [skip ci]"
	}
	return "chore(changelog): update for " + tag + " [skip ci]"
}

func packageNameFrom(ctx context.Context) string {
	if p := packageFrom(ctx); p != nil {
		return p.Name
	}
	return ""
}

func packagePathFrom(ctx context.Context) string {
	if p := packageFrom(ctx); p != nil {
		return p.Path
	}
	return ""
}

// preTagCommitPaths limits the auto-commit of pre-tag plugin changes to the
// package root in workspace mode, so unrelated tracked files that happen to be
// modified elsewhere in the repo never end up in a package's release commit.
func preTagCommitPaths(ctx context.Context) []string {
	if p := packageFrom(ctx); p != nil {
		return []string{p.Path}
	}
	return nil
}
