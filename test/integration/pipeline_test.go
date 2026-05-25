// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package integration provides end-to-end integration tests for semrel.
// These tests create real Git repositories in temp directories and exercise
// the full pipeline: commits → version calculation → tag creation.
//
// Tests are skipped if git is not available on the PATH.
// See: https://github.com/SemRels/semrel/issues/21
package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitpkg "github.com/GoSemantics/semrel/pkg/git"
	"github.com/GoSemantics/semrel/pkg/commits"
	"github.com/GoSemantics/semrel/pkg/semver"
)

// requireGit skips the test if git is not available on the PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH — skipping integration test")
	}
}

// initRepo creates a temporary git repository with an initial commit.
// Returns the repo path and a cleanup function.
func initRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Set minimal git identity so commits work in CI
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "--initial-branch=main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	// Initial commit
	readmePath := filepath.Join(dir, "README.md")
	os.WriteFile(readmePath, []byte("# Test\n"), 0o644)
	run("add", ".")
	run("commit", "-m", "chore: initial commit")

	cleanup := func() { os.RemoveAll(dir) }
	return dir, cleanup
}

// addCommit adds a file and commits it with the given message.
func addCommit(t *testing.T, dir, file, msg string) {
	t.Helper()
	path := filepath.Join(dir, file)
	os.WriteFile(path, []byte("content\n"), 0o644)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", file)
	run("commit", "-m", msg)
}

// addTag creates a lightweight tag at HEAD.
func addTag(t *testing.T, dir, tag string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "tag", tag)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag: %v\n%s", err, out)
	}
}

// TestGitRepository_OpenAndBranch verifies that OpenRepository works on a real repo.
func TestGitRepository_OpenAndBranch(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	repo, err := gitpkg.OpenRepository(dir)
	if err != nil {
		t.Fatalf("OpenRepository: %v", err)
	}
	if repo.Path == "" {
		t.Error("expected non-empty repo Path")
	}

	branch, err := repo.CurrentBranch(context.Background())
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

// TestGitRepository_LastTag_NoTags verifies that LastTag returns "" when no tags exist.
func TestGitRepository_LastTag_NoTags(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	repo, _ := gitpkg.OpenRepository(dir)
	tag, err := repo.LastTag(context.Background())
	if err != nil {
		t.Fatalf("LastTag: %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag, got %q", tag)
	}
}

// TestGitRepository_LastTag_WithTag verifies that LastTag returns the most recent tag.
func TestGitRepository_LastTag_WithTag(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "newfile.txt", "feat: add newfile")

	repo, _ := gitpkg.OpenRepository(dir)
	tag, err := repo.LastTag(context.Background())
	if err != nil {
		t.Fatalf("LastTag: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %q", tag)
	}
}

// TestGitRepository_CommitsSince verifies commit log reading.
func TestGitRepository_CommitsSince(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "a.txt", "feat: add feature A")
	addCommit(t, dir, "b.txt", "fix: fix bug B")

	repo, _ := gitpkg.OpenRepository(dir)
	msgs, err := repo.CommitsSince(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CommitsSince: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 commits, got %d: %v", len(msgs), msgs)
	}
	// Should contain both messages
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "feat: add feature A") {
		t.Error("missing feat commit")
	}
	if !strings.Contains(joined, "fix: fix bug B") {
		t.Error("missing fix commit")
	}
}

// TestGitRepository_CommitsSince_Empty verifies that no commits returns empty slice.
func TestGitRepository_CommitsSince_Empty(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, dir, "v1.0.0")

	repo, _ := gitpkg.OpenRepository(dir)
	msgs, err := repo.CommitsSince(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CommitsSince: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 commits, got %d", len(msgs))
	}
}

// TestGitRepository_CreateTag verifies tag creation.
func TestGitRepository_CreateTag(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	repo, _ := gitpkg.OpenRepository(dir)
	if err := repo.CreateTag(context.Background(), "v1.0.0", "Release v1.0.0"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	tag, err := repo.LastTag(context.Background())
	if err != nil {
		t.Fatalf("LastTag after CreateTag: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %q", tag)
	}
}

// TestFullPipeline_FeatCommit exercises the full release pipeline end-to-end.
func TestFullPipeline_FeatCommit(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	// Start at v1.0.0
	addTag(t, dir, "v1.0.0")
	addCommit(t, dir, "feat.txt", "feat: add new feature")

	repo, _ := gitpkg.OpenRepository(dir)
	ctx := context.Background()

	lastTag, _ := repo.LastTag(ctx)
	if lastTag != "v1.0.0" {
		t.Fatalf("expected v1.0.0, got %q", lastTag)
	}

	rawMsgs, _ := repo.CommitsSince(ctx, lastTag)
	if len(rawMsgs) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(rawMsgs))
	}

	parser := commits.NewParser()
	parsed := parser.ParseAll(rawMsgs)
	if len(parsed) != 1 {
		t.Fatalf("expected 1 parsed commit, got %d", len(parsed))
	}

	var hasFeat, hasFix, hasBreaking bool
	for _, c := range parsed {
		switch c.Type {
		case "feat":
			hasFeat = true
		case "fix", "perf", "revert":
			hasFix = true
		}
		if c.IsBreakingChange {
			hasBreaking = true
		}
	}

	calc := semver.NewCalculator()
	current, _ := semver.ParseVersion("1.0.0")
	next := calc.NextVersion(current, hasFeat, hasFix, hasBreaking)
	if next == nil {
		t.Fatal("expected next version, got nil")
	}
	if next.String() != "1.1.0" {
		t.Errorf("expected 1.1.0, got %s", next.String())
	}

	// Create the tag
	nextTag := "v" + next.String()
	if err := repo.CreateTag(ctx, nextTag, "Release "+nextTag); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	tag, _ := repo.LastTag(ctx)
	if tag != nextTag {
		t.Errorf("expected %s as last tag, got %q", nextTag, tag)
	}
}

// TestFullPipeline_BreakingChange verifies that a breaking commit triggers major bump.
func TestFullPipeline_BreakingChange(t *testing.T) {
	requireGit(t)
	dir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, dir, "v1.2.3")
	addCommit(t, dir, "break.txt", "feat!: remove legacy API\n\nBREAKING CHANGE: legacy endpoints removed")

	repo, _ := gitpkg.OpenRepository(dir)
	ctx := context.Background()

	rawMsgs, _ := repo.CommitsSince(ctx, "v1.2.3")
	parser := commits.NewParser()
	parsed := parser.ParseAll(rawMsgs)

	var hasFeat, hasFix, hasBreaking bool
	for _, c := range parsed {
		switch c.Type {
		case "feat":
			hasFeat = true
		case "fix":
			hasFix = true
		}
		if c.IsBreakingChange {
			hasBreaking = true
		}
	}

	if !hasBreaking {
		t.Error("expected breaking change to be detected")
	}

	calc := semver.NewCalculator()
	current, _ := semver.ParseVersion("1.2.3")
	next := calc.NextVersion(current, hasFeat, hasFix, hasBreaking)
	if next == nil {
		t.Fatal("expected next version")
	}
	if next.String() != "2.0.0" {
		t.Errorf("expected 2.0.0 for breaking change, got %s", next.String())
	}
}
