// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRepositoryReturnsRepoRoot(t *testing.T) {
	repoDir := initGitRepository(t)
	commitFile(t, repoDir, "README.md", "hello\n", "docs: add readme")

	nested := filepath.Join(repoDir, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	repo, err := OpenRepository(nested)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}
	gotPath := filepath.Clean(repo.Path)
	wantPath := filepath.Clean(repoDir)
	if gotPath != wantPath {
		t.Fatalf("OpenRepository() path = %q, want %q", gotPath, wantPath)
	}
}

func TestOpenRepositoryReturnsErrorForNonGitDirectory(t *testing.T) {
	_, err := OpenRepository(t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepositoryOperations(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)

	commitFile(t, repoDir, "app.txt", "first\n", "feat: initial release")
	runGit(t, repoDir, "tag", "v1.0.0")
	commitFile(t, repoDir, "app.txt", "second\n", "fix: patch bug")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "main" {
		t.Fatalf("CurrentBranch() = %q, want main", branch)
	}

	lastTag, err := repo.LastTag(ctx)
	if err != nil {
		t.Fatalf("LastTag() error = %v", err)
	}
	if lastTag != "v1.0.0" {
		t.Fatalf("LastTag() = %q, want v1.0.0", lastTag)
	}

	messages, err := repo.CommitsSince(ctx, lastTag)
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("CommitsSince() len = %d, want 1", len(messages))
	}
	if !strings.Contains(messages[0], "fix: patch bug") {
		t.Fatalf("CommitsSince() = %#v, want fix commit", messages)
	}

	allMessages, err := repo.CommitsSince(ctx, "")
	if err != nil {
		t.Fatalf("CommitsSince(all) error = %v", err)
	}
	if len(allMessages) != 2 {
		t.Fatalf("CommitsSince(all) len = %d, want 2", len(allMessages))
	}

	if err := repo.CreateTag(ctx, "v1.0.1", "Release v1.0.1"); err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}

	headTag, err := repo.LastTag(ctx)
	if err != nil {
		t.Fatalf("LastTag() after CreateTag error = %v", err)
	}
	if headTag != "v1.0.1" {
		t.Fatalf("LastTag() after CreateTag = %q, want v1.0.1", headTag)
	}
}

func TestLastTagReturnsEmptyWhenRepositoryHasNoTags(t *testing.T) {
	repoDir := initGitRepository(t)
	commitFile(t, repoDir, "README.md", "hello\n", "docs: first commit")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	lastTag, err := repo.LastTag(context.Background())
	if err != nil {
		t.Fatalf("LastTag() error = %v", err)
	}
	if lastTag != "" {
		t.Fatalf("LastTag() = %q, want empty string", lastTag)
	}
}

func TestCurrentBranch_DetachedHead_CIFallback(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)
	commitFile(t, repoDir, "f.txt", "x\n", "feat: init")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	// Detach HEAD so git reports "HEAD".
	runGit(t, repoDir, "checkout", "--detach")

	tests := []struct {
		name    string
		envVars map[string]string
		want    string
	}{
		{
			name:    "GitLab MR source branch",
			envVars: map[string]string{"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature/my-work"},
			want:    "feature/my-work",
		},
		{
			name:    "GitLab commit branch",
			envVars: map[string]string{"CI_COMMIT_BRANCH": "main"},
			want:    "main",
		},
		{
			name:    "GitLab ref name",
			envVars: map[string]string{"CI_COMMIT_REF_NAME": "develop"},
			want:    "develop",
		},
		{
			name:    "GitHub PR head ref",
			envVars: map[string]string{"GITHUB_HEAD_REF": "feat/pr-branch"},
			want:    "feat/pr-branch",
		},
		{
			name:    "GitHub ref name",
			envVars: map[string]string{"GITHUB_REF_NAME": "main"},
			want:    "main",
		},
		{
			name:    "generic GIT_BRANCH",
			envVars: map[string]string{"GIT_BRANCH": "release/1.0"},
			want:    "release/1.0",
		},
		{
			name:    "no CI env set — returns HEAD",
			envVars: map[string]string{},
			want:    "HEAD",
		},
	}

	ciEnvVars := []string{
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
		"CI_COMMIT_BRANCH",
		"CI_COMMIT_REF_NAME",
		"GITHUB_HEAD_REF",
		"GITHUB_REF_NAME",
		"GIT_BRANCH",
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear all CI vars, then set only the ones for this test case.
			for _, v := range ciEnvVars {
				t.Setenv(v, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			branch, err := repo.CurrentBranch(ctx)
			if err != nil {
				t.Fatalf("CurrentBranch() error = %v", err)
			}
			if branch != tc.want {
				t.Fatalf("CurrentBranch() = %q, want %q", branch, tc.want)
			}
		})
	}
}

func initGitRepository(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "semrel@example.com")
	runGit(t, dir, "config", "user.name", "semrel tests")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "config", "tag.gpgSign", "false")
	return dir
}

func commitFile(t *testing.T, dir, name, contents, message string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-m", message)
	runGit(t, dir, "branch", "-M", "main")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
