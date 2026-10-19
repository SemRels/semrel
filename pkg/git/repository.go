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
	"path/filepath"
	"strings"

	"github.com/SemRels/semrel/pkg/semver"
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

// LastTag returns the highest version tag reachable from HEAD whose name is
// tagPrefix followed by a semantic version (e.g. "v1.2.3" or "aws-a-v1.2.3").
// Tags with a different prefix, or that do not parse as semver after the
// prefix, are ignored, so packages in a monorepo never see each other's tags.
// Returns an empty string if no matching tag exists yet (first release).
func (r *Repository) LastTag(ctx context.Context, tagPrefix string) (string, error) {
	// An empty repository has no HEAD and therefore no tags to consider.
	if err := exec.CommandContext(ctx, "git", "-C", r.Path, "rev-parse", "--verify", "--quiet", "HEAD").Run(); err != nil {
		return "", nil
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "tag", "--merged", "HEAD")
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git tag --merged HEAD: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var (
		bestTag string
		best    *semver.Version
	)
	for _, tag := range strings.Split(string(out), "\n") {
		tag = strings.TrimSpace(tag)
		if tag == "" || !strings.HasPrefix(tag, tagPrefix) {
			continue
		}
		rest := tag[len(tagPrefix):]
		// The version must start right after the prefix.
		if rest == "" || rest[0] < '0' || rest[0] > '9' {
			continue
		}
		v, perr := semver.ParseVersion(rest)
		if perr != nil {
			continue
		}
		if best == nil || v.Compare(best) > 0 {
			best, bestTag = v, tag
		}
	}
	return bestTag, nil
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
//
// When paths are given, only commits that change at least one file under one of
// those paths are returned (monorepo packages). Paths may be absolute or
// relative to the repository root; they must lie inside the repository.
func (r *Repository) CommitsDetailedSince(ctx context.Context, ref string, paths ...string) ([]Commit, error) {
	args := []string{"-C", r.Path, "log"}
	if ref != "" {
		args = append(args, fmt.Sprintf("%s..HEAD", ref))
	}
	args = append(args, "--format=%H%x00%an%x00%ae%x00%B%x00%x00")
	if len(paths) > 0 {
		specs, err := r.pathspecs(paths)
		if err != nil {
			return nil, err
		}
		args = append(args, "--")
		args = append(args, specs...)
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

// pathspecs converts paths into literal, repository-root-relative git pathspecs.
func (r *Repository) pathspecs(paths []string) ([]string, error) {
	root, err := filepath.EvalSymlinks(r.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving repository root: %w", err)
	}
	specs := make([]string, 0, len(paths))
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, abs)
		} else if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
			abs = resolved
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("path %q is outside the repository %s", p, r.Path)
		}
		// The repository root itself means "everything": no restriction.
		if rel == "." {
			return []string{":(top)"}, nil
		}
		specs = append(specs, ":(top,literal)"+filepath.ToSlash(rel))
	}
	return specs, nil
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
