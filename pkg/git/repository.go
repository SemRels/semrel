// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package git provides Git repository operations.
// See: https://github.com/SemRels/semrel/issues/5
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repository represents a Git repository.
type Repository struct {
	Path string // path to the repository root
}

// Commit holds commit metadata used by release plugins.
type Commit struct {
	Hash        string
	Message     string
	AuthorName  string
	AuthorEmail string
}

// OpenRepository opens a Git repository at the given path.
// Issue: https://github.com/SemRels/semrel/issues/5
func OpenRepository(path string) (*Repository, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository at %s: %w", path, err)
	}
	root := strings.TrimSpace(string(out))
	return &Repository{Path: root}, nil
}

// LastTag returns the most recent annotated or lightweight tag reachable from HEAD.
// Returns an empty string if no tags exist yet (first release).
func (r *Repository) LastTag(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "describe", "--tags", "--abbrev=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		// No tags found — this is a first release scenario
		if strings.Contains(stderr.String(), "No names found") ||
			strings.Contains(stderr.String(), "No tags can describe") ||
			strings.Contains(stderr.String(), "No annotated tags") {
			return "", nil
		}
		// Try lightweight tags too
		cmd2 := exec.CommandContext(ctx, "git", "-C", r.Path, "describe", "--tags", "--abbrev=0", "--always")
		out2, err2 := cmd2.Output()
		if err2 != nil {
			return "", nil // no tags at all
		}
		tag := strings.TrimSpace(string(out2))
		// If it looks like a SHA (no dots, no v prefix), there are no version tags
		if !strings.Contains(tag, ".") && !strings.HasPrefix(tag, "v") {
			return "", nil
		}
		return tag, nil
	}
	return strings.TrimSpace(string(out)), nil
}

// CommitsSince returns raw commit messages since the given ref (exclusive).
// If ref is empty, returns all commits.
func (r *Repository) CommitsSince(ctx context.Context, ref string) ([]string, error) {
	var args []string
	if ref == "" {
		args = []string{"-C", r.Path, "log", "--format=%B%x00"}
	} else {
		args = []string{"-C", r.Path, "log", fmt.Sprintf("%s..HEAD", ref), "--format=%B%x00"}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	raw := string(out)
	parts := strings.Split(raw, "\x00")
	var messages []string
	for _, p := range parts {
		msg := strings.TrimSpace(p)
		if msg != "" {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

// CommitsDetailedSince returns commit metadata since the given ref (exclusive).
// If ref is empty, returns all commits reachable from HEAD.
func (r *Repository) CommitsDetailedSince(ctx context.Context, ref string) ([]Commit, error) {
	var args []string
	if ref == "" {
		args = []string{"-C", r.Path, "log", "--format=%H%x00%an%x00%ae%x00%B%x00%x00"}
	} else {
		args = []string{"-C", r.Path, "log", fmt.Sprintf("%s..HEAD", ref), "--format=%H%x00%an%x00%ae%x00%B%x00%x00"}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	raw := string(out)
	records := strings.Split(raw, "\x00\x00")
	commits := make([]Commit, 0, len(records))
	for _, record := range records {
		record = strings.TrimSuffix(record, "\x00")
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.SplitN(record, "\x00", 4)
		if len(fields) != 4 {
			return nil, fmt.Errorf("unexpected git log record format")
		}
		message := strings.TrimSpace(fields[3])
		if message == "" {
			continue
		}
		commits = append(commits, Commit{
			Hash:        strings.TrimSpace(fields[0]),
			AuthorName:  strings.TrimSpace(fields[1]),
			AuthorEmail: strings.TrimSpace(fields[2]),
			Message:     message,
		})
	}
	return commits, nil
}

// CommitCountsByAuthorEmail returns commit counts across the full repository
// history keyed by normalized (lowercase) author email.
func (r *Repository) CommitCountsByAuthorEmail(ctx context.Context) (map[string]int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "log", "--all", "--format=%ae%x00")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log --all: %w", err)
	}

	counts := map[string]int{}
	for _, part := range strings.Split(string(out), "\x00") {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" {
			continue
		}
		counts[email]++
	}
	return counts, nil
}

// CurrentBranch returns the name of the currently checked-out branch.
// In detached-HEAD environments (e.g. GitLab CI merge-request pipelines,
// GitHub Actions pull-request workflows) git returns the literal string "HEAD".
// When that happens, the function falls back to well-known CI environment
// variables so that semrel can still determine the correct release branch:
//
//	GitLab MR source branch : CI_MERGE_REQUEST_SOURCE_BRANCH_NAME
//	GitLab branch/tag ref   : CI_COMMIT_REF_NAME
//	GitHub PR head branch   : GITHUB_HEAD_REF
//	GitHub push/tag ref     : GITHUB_REF_NAME
//	Generic fallback        : GIT_BRANCH (set by Jenkins and many others)
func (r *Repository) CurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))

	if branch != "HEAD" {
		return branch, nil
	}

	// Detached HEAD — probe CI environment variables in preference order.
	for _, envVar := range []string{
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", // GitLab MR pipeline
		"CI_COMMIT_BRANCH",                    // GitLab branch pipeline
		"CI_COMMIT_REF_NAME",                  // GitLab general (branch or tag name)
		"GITHUB_HEAD_REF",                     // GitHub Actions PR
		"GITHUB_REF_NAME",                     // GitHub Actions push/tag
		"GIT_BRANCH",                          // Jenkins, Gitea Actions, others
	} {
		if v := strings.TrimSpace(os.Getenv(envVar)); v != "" && v != "HEAD" {
			return v, nil
		}
	}

	return "HEAD", nil
}

// CreateTag creates an annotated tag at HEAD.
func (r *Repository) CreateTag(ctx context.Context, tag, message string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "tag", "-a", tag, "-m", message)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating tag %s: %w\n%s", tag, err, out)
	}
	return nil
}

// TagExists reports whether the given tag exists locally.
func (r *Repository) TagExists(ctx context.Context, tag string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "tag", "-l", tag)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("checking tag %s: %w", tag, err)
	}
	return strings.TrimSpace(string(out)) == tag, nil
}

// CommitFiles stages the given file paths and creates a commit with the given message.
// It skips empty staging areas gracefully (nothing to commit).
func (r *Repository) CommitFiles(ctx context.Context, files []string, message string) error {
	// Stage files
	addArgs := append([]string{"-C", r.Path, "add", "--"}, files...)
	if out, err := exec.CommandContext(ctx, "git", addArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, out)
	}
	// Check if there is actually something staged
	diffOut, err := exec.CommandContext(ctx, "git", "-C", r.Path, "diff", "--cached", "--name-only").Output()
	if err != nil || strings.TrimSpace(string(diffOut)) == "" {
		return nil // nothing to commit — idempotent
	}
	commitArgs := []string{"-C", r.Path, "commit", "--no-verify", "-m", message}
	if out, err := exec.CommandContext(ctx, "git", commitArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, out)
	}
	return nil
}

// CommitModifiedTrackedFiles stages all modified tracked files and commits them.
// This is used after pre-tag plugins run to capture any version-file changes
// so the tagged commit already contains the correct version.
// Returns (false, nil) if there are no changes to commit.
func (r *Repository) CommitModifiedTrackedFiles(ctx context.Context, message string) (bool, error) {
	// git add -u stages only modified/deleted tracked files (not new untracked files)
	if out, err := exec.CommandContext(ctx, "git", "-C", r.Path, "add", "-u").CombinedOutput(); err != nil {
		return false, fmt.Errorf("git add -u: %w\n%s", err, out)
	}
	diffOut, err := exec.CommandContext(ctx, "git", "-C", r.Path, "diff", "--cached", "--name-only").Output()
	if err != nil || strings.TrimSpace(string(diffOut)) == "" {
		return false, nil // nothing staged
	}
	commitArgs := []string{"-C", r.Path, "commit", "--no-verify", "-m", message}
	if out, err := exec.CommandContext(ctx, "git", commitArgs...).CombinedOutput(); err != nil {
		return false, fmt.Errorf("git commit: %w\n%s", err, out)
	}
	return true, nil
}
