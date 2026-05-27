// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package git provides Git repository operations.
// See: https://github.com/SemRels/semrel/issues/5
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Repository represents a Git repository.
type Repository struct {
	Path string // path to the repository root
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

// CurrentBranch returns the name of the currently checked-out branch.
func (r *Repository) CurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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
