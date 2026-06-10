// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package changelog

import (
	"strings"
	"testing"
	"time"

	"github.com/SemRels/semrel/pkg/commits"
)

var testDate = time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

func TestNewPackageGenerator(t *testing.T) {
	pg := NewPackageGenerator("api", "packages/api")
	if pg.PackageName != "api" || pg.PackagePath != "packages/api" {
		t.Errorf("unexpected PackageGenerator: %+v", pg)
	}
}

func TestFilterCommits_ByPath(t *testing.T) {
	pg := NewPackageGenerator("api", "packages/api")
	cs := []CommitWithFiles{
		{Commit: &commits.Commit{Type: "feat", Description: "add endpoint"}, ChangedFiles: []string{"packages/api/handler.go"}},
		{Commit: &commits.Commit{Type: "fix", Description: "core fix"}, ChangedFiles: []string{"packages/core/util.go"}},
		{Commit: &commits.Commit{Type: "docs", Description: "readme"}, ChangedFiles: []string{"packages/api/README.md", "docs/index.md"}},
	}
	got := pg.FilterCommits(cs)
	if len(got) != 2 {
		t.Fatalf("expected 2 commits for api, got %d", len(got))
	}
}

func TestFilterCommits_EmptyPath(t *testing.T) {
	pg := NewPackageGenerator("all", "")
	cs := []CommitWithFiles{
		{Commit: &commits.Commit{Type: "feat", Description: "anything"}, ChangedFiles: []string{"anywhere.go"}},
	}
	got := pg.FilterCommits(cs)
	if len(got) != 1 {
		t.Fatalf("empty path should return all commits, got %d", len(got))
	}
}

func TestFilterCommits_NilCommit(t *testing.T) {
	pg := NewPackageGenerator("api", "packages/api")
	cs := []CommitWithFiles{
		{Commit: nil, ChangedFiles: []string{"packages/api/x.go"}},
	}
	got := pg.FilterCommits(cs)
	if len(got) != 0 {
		t.Fatalf("nil commit should be filtered, got %d", len(got))
	}
}

func TestGenerateEntry_FiltersCorrectly(t *testing.T) {
	pg := NewPackageGenerator("api", "packages/api")
	cs := []CommitWithFiles{
		{Commit: &commits.Commit{Type: "feat", Description: "api feature"}, ChangedFiles: []string{"packages/api/main.go"}},
		{Commit: &commits.Commit{Type: "fix", Description: "unrelated"}, ChangedFiles: []string{"packages/other/main.go"}},
	}
	entry := pg.GenerateEntry("v1.1.0", testDate, cs)
	if len(entry.Commits) != 1 {
		t.Fatalf("expected 1 filtered commit, got %d", len(entry.Commits))
	}
	if entry.Version != "v1.1.0" {
		t.Errorf("unexpected version %q", entry.Version)
	}
}

func TestRenderEntry_HasVersionAndDate(t *testing.T) {
	pg := NewPackageGenerator("api", "packages/api")
	entry := PackageEntry{
		Version: "v1.2.0",
		Date:    testDate,
		Commits: []*commits.Commit{{Type: "feat", Description: "new feature"}},
	}
	rendered := pg.RenderEntry(entry)
	if !strings.Contains(rendered, "## [v1.2.0] - 2026-04-01") {
		t.Error("expected version and date header")
	}
}

func TestRenderEntry_Empty(t *testing.T) {
	pg := NewPackageGenerator("api", "packages/api")
	entry := PackageEntry{Version: "v1.0.0", Date: testDate}
	rendered := pg.RenderEntry(entry)
	if !strings.Contains(rendered, "_No changes in this package._") {
		t.Error("expected no-changes placeholder")
	}
}

func TestRenderFull_Header(t *testing.T) {
	pg := NewPackageGenerator("my-api", "packages/api")
	entries := []PackageEntry{
		{Version: "v1.0.0", Date: testDate, Commits: []*commits.Commit{{Type: "feat", Description: "init"}}},
	}
	full := pg.RenderFull(entries)
	if !strings.Contains(full, "# Changelog — my-api") {
		t.Error("expected package-specific header")
	}
	if !strings.Contains(full, "Keep a Changelog") {
		t.Error("expected keepachangelog reference")
	}
}
