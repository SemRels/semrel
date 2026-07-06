// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	gitpkg "github.com/SemRels/semrel/pkg/git"
)

func TestBuildContributorMetadata(t *testing.T) {
	commits := []gitpkg.Commit{
		{AuthorName: "Alice", AuthorEmail: "Alice@Example.com", Message: "feat: one"},
		{AuthorName: "Bob", AuthorEmail: "bob@example.com", Message: "fix: two"},
		{AuthorName: "Alice Updated", AuthorEmail: "alice@example.com", Message: "fix: three"},
	}
	fullHistoryCounts := map[string]int{
		"alice@example.com": 2,
		"bob@example.com":   4,
	}

	got := buildContributorMetadata(commits, fullHistoryCounts)
	if len(got) != 2 {
		t.Fatalf("len(buildContributorMetadata()) = %d, want 2", len(got))
	}
	if got[0].Email != "Alice@Example.com" {
		t.Fatalf("got[0].Email = %q, want %q", got[0].Email, "Alice@Example.com")
	}
	if got[0].Commits != 2 {
		t.Fatalf("got[0].Commits = %d, want 2", got[0].Commits)
	}
	if !got[0].FirstContribution {
		t.Fatal("got[0].FirstContribution = false, want true")
	}
	if got[1].FirstContribution {
		t.Fatal("got[1].FirstContribution = true, want false")
	}
}

func TestReleaseContextEnvIncludesContributorMetadata(t *testing.T) {
	rel := ReleaseSummary{
		CurrentVersion: "v1.0.0",
		NextVersion:    "v1.1.0",
		Bump:           "minor",
		Branch:         "main",
		TagPrefix:      "v",
		Changelog:      "## v1.1.0",
		CommitMessages: []string{"feat: one", "fix: two"},
		contributors: []contributorMetadata{
			{Name: "Alice", Email: "alice@example.com", Commits: 2, FirstContribution: true},
			{Name: "Bob", Email: "bob@example.com", Commits: 1, FirstContribution: false},
		},
	}

	env := releaseContextEnv(rel, false)
	envMap := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		envMap[key] = value
	}

	if envMap["SEMREL_COMMITS"] != `["feat: one","fix: two"]` {
		t.Fatalf("SEMREL_COMMITS = %q", envMap["SEMREL_COMMITS"])
	}

	var contributors []contributorMetadata
	if err := json.Unmarshal([]byte(envMap["SEMREL_CONTRIBUTORS"]), &contributors); err != nil {
		t.Fatalf("json.Unmarshal(SEMREL_CONTRIBUTORS) error = %v", err)
	}
	if len(contributors) != 2 {
		t.Fatalf("len(SEMREL_CONTRIBUTORS) = %d, want 2", len(contributors))
	}
	if contributors[0].Name != "Alice" || contributors[0].Commits != 2 || !contributors[0].FirstContribution {
		t.Fatalf("contributors[0] = %#v, want Alice with 2 commits and firstContribution=true", contributors[0])
	}
}
