// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Per-package changelog generation for monorepo repositories.
//
// In a monorepo each package may have its own CHANGELOG.md that only includes
// commits touching files under that package's subdirectory. This file extends
// the changelog package with path-based commit filtering and per-package
// rendering.
//
// See: https://github.com/SemRels/semrel/issues/61
package changelog

import (
	"fmt"
	"strings"
	"time"

	"github.com/SemRels/semrel/pkg/commits"
)

// CommitWithFiles wraps a commits.Commit with the list of files it changed.
// This is used for per-package filtering in monorepo mode.
type CommitWithFiles struct {
	*commits.Commit
	// ChangedFiles is the list of file paths modified by this commit.
	ChangedFiles []string
}

// PackageEntry is a versioned changelog entry scoped to a monorepo package.
type PackageEntry struct {
	// PackageName is the display name of the package (e.g. "packages/api").
	PackageName string
	// Version is the package version string (e.g. "v1.2.0").
	Version string
	// Date is the release date.
	Date time.Time
	// Commits are the commits included in this version, already filtered to
	// this package's path.
	Commits []*commits.Commit
}

// PackageGenerator generates changelogs for a specific monorepo package by
// filtering commits to only those that touch the package directory.
type PackageGenerator struct {
	// PackageName is the display name of the package.
	PackageName string
	// PackagePath is the root path of the package (relative to repo root).
	// Only commits that modify files under this path are included.
	PackagePath string
}

// NewPackageGenerator creates a PackageGenerator for a specific package.
func NewPackageGenerator(packageName, packagePath string) *PackageGenerator {
	return &PackageGenerator{
		PackageName: packageName,
		PackagePath: strings.TrimRight(packagePath, "/"),
	}
}

// FilterCommits returns only the commits whose ChangedFiles include at least
// one file under PackagePath.
func (pg *PackageGenerator) FilterCommits(cs []CommitWithFiles) []CommitWithFiles {
	if pg.PackagePath == "" {
		return cs
	}
	prefix := pg.PackagePath + "/"
	var filtered []CommitWithFiles
	for _, c := range cs {
		if c.Commit == nil {
			continue
		}
		for _, f := range c.ChangedFiles {
			if strings.HasPrefix(f, prefix) {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered
}

// GenerateEntry creates a PackageEntry for a single release.
// The commits are filtered to this package's path automatically.
func (pg *PackageGenerator) GenerateEntry(version string, date time.Time, cs []CommitWithFiles) PackageEntry {
	filtered := pg.FilterCommits(cs)
	plain := make([]*commits.Commit, 0, len(filtered))
	for _, c := range filtered {
		plain = append(plain, c.Commit)
	}
	return PackageEntry{
		PackageName: pg.PackageName,
		Version:     version,
		Date:        date,
		Commits:     plain,
	}
}

// RenderEntry renders a single PackageEntry as a Keep-a-Changelog Markdown section.
func (pg *PackageGenerator) RenderEntry(entry PackageEntry) string {
	gen := NewGenerator()
	dateStr := entry.Date.UTC().Format("2006-01-02")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## [%s] - %s\n\n", entry.Version, dateStr))
	if len(entry.Commits) == 0 {
		sb.WriteString("_No changes in this package._\n")
		return sb.String()
	}
	// Reuse the core Generate logic and strip its version header line
	body := gen.Generate(entry.Version, entry.Commits)
	lines := strings.SplitN(body, "\n", 3)
	if len(lines) >= 3 {
		sb.WriteString(lines[2])
	} else {
		sb.WriteString(body)
	}
	return sb.String()
}

// RenderFull renders a complete per-package CHANGELOG.md file.
func (pg *PackageGenerator) RenderFull(entries []PackageEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Changelog — %s\n\n", pg.PackageName))
	sb.WriteString("All notable changes to this package will be documented in this file.\n\n")
	sb.WriteString("The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).\n\n")
	for _, e := range entries {
		sb.WriteString(pg.RenderEntry(e))
		sb.WriteByte('\n')
	}
	return sb.String()
}
