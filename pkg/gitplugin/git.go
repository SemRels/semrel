// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package gitplugin provides a built-in git plugin for semrel.
// It creates git tags, commits, and pushes to the remote repository as part
// of the release process.
package gitplugin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Config holds the git plugin configuration.
type Config struct {
	// TagName is the tag to create (e.g., "v1.2.3"). Supports {version} placeholder.
	TagName string
	// TagMessage is the annotated tag message. If empty, a lightweight tag is created.
	TagMessage string
	// CommitMessage is the commit message for release commits (e.g., updating CHANGELOG).
	CommitMessage string
	// Remote is the git remote to push to (defaults to "origin").
	Remote string
	// Branch is the branch to push to (defaults to "main").
	Branch string
	// Files is the list of files to stage and commit before tagging.
	Files []string
	// SignTag enables GPG signing of the tag.
	SignTag bool
	// SignedOffBy adds a Signed-off-by trailer to commits (for DCO compliance).
	SignedOffBy bool
}

// Plugin is the git built-in plugin.
type Plugin struct {
	cfg    Config
	runner cmdRunner
}

// cmdRunner abstracts exec.Command for testing.
type cmdRunner interface {
	run(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type realRunner struct{}

func (realRunner) run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// NewPlugin creates a git Plugin with the given configuration.
func NewPlugin(cfg Config) *Plugin {
	if cfg.Remote == "" {
		cfg.Remote = "origin"
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	return &Plugin{cfg: cfg, runner: realRunner{}}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "git" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Validate checks the plugin configuration.
func (p *Plugin) Validate() error {
	if p.cfg.TagName == "" {
		return fmt.Errorf("git plugin: tag_name is required")
	}
	return nil
}

// CreateTag creates a git tag at the current HEAD.
func (p *Plugin) CreateTag(ctx context.Context, dir, version string) error {
	tagName := expandPlaceholder(p.cfg.TagName, version)

	args := []string{"tag"}
	if p.cfg.SignTag {
		args = append(args, "-s")
	}
	if p.cfg.TagMessage != "" {
		args = append(args, "-a", "-m", expandPlaceholder(p.cfg.TagMessage, version))
	}
	args = append(args, tagName)

	if _, err := p.runner.run(ctx, dir, "git", args...); err != nil {
		return fmt.Errorf("git plugin: create tag %q: %w", tagName, err)
	}
	return nil
}

// CommitFiles stages the configured files and creates a commit.
func (p *Plugin) CommitFiles(ctx context.Context, dir, version string) error {
	if len(p.cfg.Files) == 0 {
		return nil
	}

	// Stage files
	addArgs := append([]string{"add"}, p.cfg.Files...)
	if _, err := p.runner.run(ctx, dir, "git", addArgs...); err != nil {
		return fmt.Errorf("git plugin: stage files: %w", err)
	}

	// Commit
	msg := expandPlaceholder(p.cfg.CommitMessage, version)
	if msg == "" {
		msg = fmt.Sprintf("chore: release %s", version)
	}
	commitArgs := []string{"commit", "-m", msg}
	if p.cfg.SignedOffBy {
		commitArgs = append(commitArgs, "--signoff")
	}
	if _, err := p.runner.run(ctx, dir, "git", commitArgs...); err != nil {
		return fmt.Errorf("git plugin: commit: %w", err)
	}
	return nil
}

// Push pushes the branch and tag to the remote.
func (p *Plugin) Push(ctx context.Context, dir, version string) error {
	tagName := expandPlaceholder(p.cfg.TagName, version)

	// Push branch
	if _, err := p.runner.run(ctx, dir, "git", "push", p.cfg.Remote, p.cfg.Branch); err != nil {
		return fmt.Errorf("git plugin: push branch: %w", err)
	}

	// Push tag
	if _, err := p.runner.run(ctx, dir, "git", "push", p.cfg.Remote, tagName); err != nil {
		return fmt.Errorf("git plugin: push tag %q: %w", tagName, err)
	}
	return nil
}

// ExpandTagName returns the tag name with {version} replaced.
func ExpandTagName(template, version string) string {
	return expandPlaceholder(template, version)
}

// expandPlaceholder replaces {version} with the actual version.
func expandPlaceholder(s, version string) string {
	return strings.ReplaceAll(s, "{version}", version)
}
