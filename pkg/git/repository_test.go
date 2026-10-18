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

	lastTag, err := repo.LastTag(ctx, "v")
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

	headTag, err := repo.LastTag(ctx, "v")
	if err != nil {
		t.Fatalf("LastTag() after CreateTag error = %v", err)
	}
	if headTag != "v1.0.1" {
		t.Fatalf("LastTag() after CreateTag = %q, want v1.0.1", headTag)
	}
}

func TestCommitsDetailedSinceAndCommitCountsByAuthorEmail(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)

	commitFileAsAuthor(t, repoDir, "README.md", "base\n", "feat: bootstrap", "Existing Author", "existing@example.com")
	runGit(t, repoDir, "tag", "v1.0.0")
	commitFileAsAuthor(t, repoDir, "feature.txt", "one\n", "feat: add feature", "New Contributor", "New@Example.com")
	commitFileAsAuthor(t, repoDir, "feature.txt", "two\n", "fix: tighten feature", "New Contributor", "new@example.com")
	commitFileAsAuthor(t, repoDir, "README.md", "update\n", "fix: patch docs", "Existing Author", "Existing@Example.com")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	commits, err := repo.CommitsDetailedSince(ctx, "v1.0.0")
	if err != nil {
		t.Fatalf("CommitsDetailedSince() error = %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("CommitsDetailedSince() len = %d, want 3", len(commits))
	}
	if commits[0].AuthorEmail != "Existing@Example.com" {
		t.Fatalf("CommitsDetailedSince()[0].AuthorEmail = %q, want %q", commits[0].AuthorEmail, "Existing@Example.com")
	}
	if commits[1].AuthorName != "New Contributor" {
		t.Fatalf("CommitsDetailedSince()[1].AuthorName = %q, want %q", commits[1].AuthorName, "New Contributor")
	}
	if !strings.Contains(commits[2].Message, "feat: add feature") {
		t.Fatalf("CommitsDetailedSince()[2].Message = %q, want feature commit", commits[2].Message)
	}

	counts, err := repo.CommitCountsByAuthorEmail(ctx)
	if err != nil {
		t.Fatalf("CommitCountsByAuthorEmail() error = %v", err)
	}
	if counts["new@example.com"] != 2 {
		t.Fatalf("CommitCountsByAuthorEmail()[new@example.com] = %d, want 2", counts["new@example.com"])
	}
	if counts["existing@example.com"] != 2 {
		t.Fatalf("CommitCountsByAuthorEmail()[existing@example.com] = %d, want 2", counts["existing@example.com"])
	}
}

func TestCommitsDetailedSinceFiltersByPath(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)

	commitFile(t, repoDir, "README.md", "base\n", "feat: bootstrap")
	runGit(t, repoDir, "tag", "v1.0.0")
	commitFile(t, repoDir, "pkg-a/a.txt", "a\n", "feat(a): a only")
	commitFile(t, repoDir, "pkg-b/b.txt", "b\n", "fix(b): b only")
	// A commit touching both packages must be attributed to each of them.
	for _, f := range []string{"pkg-a/a.txt", "pkg-b/b.txt"} {
		if err := os.WriteFile(filepath.Join(repoDir, f), []byte("both\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repoDir, "add", "-A")
	runGit(t, repoDir, "commit", "-m", "chore: touch both")
	commitFile(t, repoDir, ".github/ci.yml", "ci\n", "ci: repo-level")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	count := func(paths ...string) int {
		t.Helper()
		cs, err := repo.CommitsDetailedSince(ctx, "v1.0.0", paths...)
		if err != nil {
			t.Fatalf("CommitsDetailedSince(%v) error = %v", paths, err)
		}
		return len(cs)
	}

	if got := count(); got != 4 {
		t.Errorf("unscoped = %d, want 4", got)
	}
	if got := count("pkg-a"); got != 2 {
		t.Errorf("relative pkg-a = %d, want 2", got)
	}
	if got := count(filepath.Join(repoDir, "pkg-b")); got != 2 {
		t.Errorf("absolute pkg-b = %d, want 2", got)
	}
	if got := count("pkg-a", "pkg-b"); got != 3 {
		t.Errorf("pkg-a+pkg-b = %d, want 3", got)
	}
	if got := count("pkg-missing"); got != 0 {
		t.Errorf("missing path = %d, want 0", got)
	}
	if got := count(repoDir); got != 4 {
		t.Errorf("repo root = %d, want 4", got)
	}
	if _, err := repo.CommitsDetailedSince(ctx, "v1.0.0", t.TempDir()); err == nil {
		t.Error("expected error for a path outside the repository")
	}
}

func TestLastTagHonorsTagPrefix(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)

	commitFile(t, repoDir, "README.md", "base\n", "chore: init")
	runGit(t, repoDir, "tag", "-a", "aws-a-v1.0.0", "-m", "x")
	runGit(t, repoDir, "tag", "-a", "aws-b-v1.0.0", "-m", "x")
	commitFile(t, repoDir, "aws-a/a.txt", "a\n", "feat: a")
	runGit(t, repoDir, "tag", "-a", "aws-a-v1.1.0", "-m", "x")
	commitFile(t, repoDir, "aws-b/b.txt", "b\n", "feat: b")
	runGit(t, repoDir, "tag", "-a", "aws-b-v1.0.1", "-m", "x")
	// Unrelated marker tags must never be picked up.
	runGit(t, repoDir, "tag", "deploy-prod")
	runGit(t, repoDir, "tag", "aws-a-vnext")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	cases := []struct{ prefix, want string }{
		{"aws-a-v", "aws-a-v1.1.0"},
		{"aws-b-v", "aws-b-v1.0.1"},
		{"v", ""},
		{"aws-c-v", ""},
		{"aws-", ""}, // remainder "a-v1.1.0" does not start with a version
	}
	for _, tc := range cases {
		got, err := repo.LastTag(ctx, tc.prefix)
		if err != nil {
			t.Fatalf("LastTag(%q) error = %v", tc.prefix, err)
		}
		if got != tc.want {
			t.Errorf("LastTag(%q) = %q, want %q", tc.prefix, got, tc.want)
		}
	}
}

func TestLastTagPicksHighestVersionAndHandlesPrereleases(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)

	commitFile(t, repoDir, "a.txt", "1\n", "feat: one")
	runGit(t, repoDir, "tag", "v1.9.0")
	commitFile(t, repoDir, "a.txt", "2\n", "feat: two")
	runGit(t, repoDir, "tag", "v1.10.0")
	commitFile(t, repoDir, "a.txt", "3\n", "feat: three")
	runGit(t, repoDir, "tag", "v2.0.0-beta.1")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}
	got, err := repo.LastTag(ctx, "v")
	if err != nil {
		t.Fatalf("LastTag() error = %v", err)
	}
	if got != "v2.0.0-beta.1" {
		t.Errorf("LastTag() = %q, want v2.0.0-beta.1", got)
	}

	// Only tags reachable from HEAD count: v2.0.0-beta.1 is not on this branch.
	runGit(t, repoDir, "checkout", "-q", "v1.10.0")
	got, err = repo.LastTag(ctx, "v")
	if err != nil {
		t.Fatalf("LastTag() error = %v", err)
	}
	if got != "v1.10.0" {
		t.Errorf("LastTag() on older commit = %q, want v1.10.0", got)
	}
}

func TestLastTagReturnsEmptyWhenRepositoryHasNoTags(t *testing.T) {
	repoDir := initGitRepository(t)
	commitFile(t, repoDir, "README.md", "hello\n", "docs: first commit")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	lastTag, err := repo.LastTag(context.Background(), "v")
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

func commitFileAsAuthor(t *testing.T, dir, name, contents, message, authorName, authorEmail string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, dir, "add", name)
	runGitEnv(t, dir, map[string]string{
		"GIT_AUTHOR_NAME":     authorName,
		"GIT_AUTHOR_EMAIL":    authorEmail,
		"GIT_COMMITTER_NAME":  authorName,
		"GIT_COMMITTER_EMAIL": authorEmail,
	}, "commit", "-m", message)
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

func runGitEnv(t *testing.T, dir string, env map[string]string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s error: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitModifiedTrackedFilesHonorsPaths(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)
	commitFile(t, repoDir, "pkg-a/a.txt", "a\n", "feat: a")
	commitFile(t, repoDir, "pkg-b/b.txt", "b\n", "feat: b")
	for _, f := range []string{"pkg-a/a.txt", "pkg-b/b.txt"} {
		if err := os.WriteFile(filepath.Join(repoDir, f), []byte("changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}

	committed, err := repo.CommitModifiedTrackedFiles(ctx, "chore: scoped", filepath.Join(repoDir, "pkg-a"))
	if err != nil || !committed {
		t.Fatalf("CommitModifiedTrackedFiles() = %v, %v", committed, err)
	}
	if files := runGit(t, repoDir, "show", "--name-only", "--format=", "HEAD"); files != "pkg-a/a.txt" {
		t.Errorf("scoped commit touched %q, want only pkg-a/a.txt", files)
	}
	if status := runGit(t, repoDir, "status", "--porcelain"); !strings.Contains(status, "pkg-b/b.txt") {
		t.Errorf("pkg-b change should stay uncommitted, status = %q", status)
	}
}

func TestPackagePathspecsSharedIgnoreAndMultiPackage(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepository(t)

	commitFile(t, repoDir, "README.md", "base\n", "chore: init")
	runGit(t, repoDir, "tag", "v0.0.0")
	commitFile(t, repoDir, "pkg-a/a.txt", "a\n", "feat(a): a code")       // 1
	commitFile(t, repoDir, "pkg-a/README.md", "doc\n", "docs(a): readme") // 2: ignored only
	commitFile(t, repoDir, "yarn.lock", "lock\n", "deps: bump lock")      // 3: shared only
	commitFile(t, repoDir, ".github/ci.yml", "ci\n", "ci: workflow")      // 4: repo-level, not shared
	commitFile(t, repoDir, "pkg-b/b.txt", "b\n", "fix(b): b code")        // 5
	// 6: touches both packages and a doc in pkg-a
	for _, f := range []string{"pkg-a/a.txt", "pkg-b/b.txt", "pkg-a/README.md"} {
		if err := os.WriteFile(filepath.Join(repoDir, f), []byte("both\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repoDir, "add", "-A")
	runGit(t, repoDir, "commit", "-m", "refactor: touch both")

	repo, err := OpenRepository(repoDir)
	if err != nil {
		t.Fatalf("OpenRepository() error = %v", err)
	}
	subjects := func(pkg string, shared []string, includeShared bool, ignore []string) []string {
		t.Helper()
		specs, err := repo.PackagePathspecs(filepath.Join(repoDir, pkg), shared, includeShared, ignore)
		if err != nil {
			t.Fatalf("PackagePathspecs() error = %v", err)
		}
		cs, err := repo.CommitsDetailedSincePathspecs(ctx, "v0.0.0", specs)
		if err != nil {
			t.Fatalf("CommitsDetailedSincePathspecs(%v) error = %v", specs, err)
		}
		var out []string
		for _, c := range cs {
			out = append(out, strings.SplitN(c.Message, "\n", 2)[0])
		}
		return out
	}
	shared := []string{"yarn.lock"}

	if got := subjects("pkg-a", shared, false, nil); len(got) != 3 {
		t.Errorf("pkg-a plain = %v, want 3 commits (a code, readme, both)", got)
	}
	if got := subjects("pkg-a", shared, true, nil); len(got) != 4 {
		t.Errorf("pkg-a +shared = %v, want 4 commits", got)
	}
	// Ignoring markdown drops the docs-only commit but keeps the multi-file one.
	got := subjects("pkg-a", shared, false, []string{"**/*.md"})
	if len(got) != 2 {
		t.Errorf("pkg-a ignoring md = %v, want 2 commits", got)
	}
	for _, s := range got {
		if strings.HasPrefix(s, "docs(a)") {
			t.Errorf("docs-only commit must be ignored, got %v", got)
		}
	}
	// The commit touching both packages appears for pkg-b as well.
	if got := subjects("pkg-b", shared, false, nil); len(got) != 2 {
		t.Errorf("pkg-b = %v, want 2 commits (b code, both)", got)
	}
	// Repo-level files that are not shared never select a commit.
	for _, s := range subjects("pkg-b", shared, true, nil) {
		if strings.HasPrefix(s, "ci:") {
			t.Errorf("non-shared repo-level commit leaked into pkg-b")
		}
	}
	if _, err := repo.PackagePathspecs(t.TempDir(), nil, false, nil); err == nil {
		t.Error("expected error for a package outside the repository")
	}
}
