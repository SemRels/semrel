// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package cli provides placeholder tests for the root command.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SemRels/semrel/internal/registry"
	"github.com/SemRels/semrel/pkg/config"
	"github.com/SemRels/semrel/pkg/plugininstance"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Fatal("expected non-nil root command")
	}
	if cmd.Use != "semrel" {
		t.Errorf("expected Use=semrel, got %q", cmd.Use)
	}
}

func TestNewRootCommand_Subcommands(t *testing.T) {
	cmd := NewRootCommand()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Use] = true
	}
	if !names["release"] {
		t.Error("expected 'release' subcommand")
	}
	if !names["lint"] {
		t.Error("expected 'lint' subcommand")
	}
}

func TestNewRootCommand_Flags(t *testing.T) {
	cmd := NewRootCommand()
	if f := cmd.PersistentFlags().Lookup("dry-run"); f == nil {
		t.Error("expected --dry-run flag")
	}
	if f := cmd.PersistentFlags().Lookup("config"); f == nil {
		t.Error("expected --config flag")
	} else if f.DefValue != "" {
		t.Errorf("expected --config default to be empty for auto-discovery, got %q", f.DefValue)
	}
}

func TestIsBranchConfigured(t *testing.T) {
	tests := []struct {
		branch   string
		branches []config.BranchConfig
		want     bool
	}{
		{"main", nil, true}, // no config = all branches
		{"main", []config.BranchConfig{{Name: "main"}}, true},                 // exact match
		{"main", []config.BranchConfig{{Name: "develop"}}, false},             // not in list
		{"next", []config.BranchConfig{{Name: "main"}, {Name: "next"}}, true}, // second match
		{"feature/x", []config.BranchConfig{{Name: "main"}}, false},           // feature branch not listed
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := isBranchConfigured(tt.branch, tt.branches)
			if got != tt.want {
				t.Errorf("isBranchConfigured(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestPrependChangelog_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	if err := prependChangelog(path, "## v1.0.0\n\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "## v1.0.0") {
		t.Errorf("changelog not written: %s", string(data))
	}
}

func TestPrependChangelog_Prepends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	os.WriteFile(path, []byte("## v0.1.0\n\nOld entry.\n"), 0o644)

	if err := prependChangelog(path, "## v1.0.0\n\nNew entry.\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.HasPrefix(content, "## v1.0.0") {
		t.Errorf("new entry not prepended: %s", content)
	}
	if !strings.Contains(content, "## v0.1.0") {
		t.Errorf("old entry missing: %s", content)
	}
	// New entry must come before old entry
	newIdx := strings.Index(content, "v1.0.0")
	oldIdx := strings.Index(content, "v0.1.0")
	if newIdx >= oldIdx {
		t.Error("new entry is not before old entry")
	}
}

func TestPrependChangelog_PermissionError(t *testing.T) {
	err := prependChangelog("/nonexistent/path/CHANGELOG.md", "content")
	if err == nil {
		t.Error("expected error writing to invalid path")
	}
}

func TestNewRootCommand_DryRunDefault(t *testing.T) {
	cmd := NewRootCommand()
	f := cmd.PersistentFlags().Lookup("dry-run")
	if f == nil {
		t.Fatal("expected --dry-run flag")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default dry-run=false, got %q", f.DefValue)
	}
}

func TestNewRootCommand_OutputFlag(t *testing.T) {
	cmd := NewRootCommand()
	f := cmd.PersistentFlags().Lookup("output")
	if f == nil {
		t.Fatal("expected --output flag")
	}
	if f.DefValue != "text" {
		t.Errorf("expected default output=text, got %q", f.DefValue)
	}
}

func TestPrintSummary_TextFormat(t *testing.T) {
	// printSummary with "text" and released=false should produce no output
	s := ReleaseSummary{Released: false}
	if err := printSummary(s, "text"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReleaseSummary_Fields(t *testing.T) {
	s := ReleaseSummary{
		Released:    true,
		NextVersion: "v1.2.3",
		Bump:        "minor",
		Commits:     5,
		Breaking:    false,
		Features:    true,
	}
	if s.NextVersion != "v1.2.3" {
		t.Errorf("wrong NextVersion: %s", s.NextVersion)
	}
	if s.Commits != 5 {
		t.Errorf("wrong Commits: %d", s.Commits)
	}
	if !s.Released {
		t.Error("expected Released=true")
	}
}

// ---------------------------------------------------------------------------
// commitlint tests
// ---------------------------------------------------------------------------

func TestCommitlintCommand_Exists(t *testing.T) {
	cmd := NewRootCommand()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Use] = true
	}
	found := false
	for n := range names {
		if strings.HasPrefix(n, "commitlint") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'commitlint' subcommand")
	}
}

func TestRunCommitlint_ValidMessages(t *testing.T) {
	err := runCommitlint(context.TODO(), []string{
		"feat: add feature",
		"fix(auth): patch login bug",
		"chore!: drop support",
	}, "", "HEAD", false, "text")
	if err != nil {
		t.Errorf("expected no error for valid messages, got: %v", err)
	}
}

func TestRunCommitlint_InvalidMessage(t *testing.T) {
	err := runCommitlint(context.TODO(), []string{"not a conventional commit"}, "", "HEAD", false, "text")
	if err == nil {
		t.Fatal("expected error for invalid message")
	}
	if !strings.Contains(err.Error(), "1 invalid") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCommitlint_MixedMessages(t *testing.T) {
	err := runCommitlint(context.TODO(), []string{
		"feat: valid",
		"bad message without type",
	}, "", "HEAD", false, "text")
	if err == nil {
		t.Fatal("expected error for mixed messages")
	}
	if !strings.Contains(err.Error(), "1 invalid") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCommitlint_JSONOutput(t *testing.T) {
	// JSON output for valid messages should not error
	err := runCommitlint(context.TODO(), []string{"feat: valid"}, "", "HEAD", false, "json")
	if err != nil {
		t.Errorf("expected no error for valid JSON output, got: %v", err)
	}
}

// TestRunCommitlint_NoArgs_DefaultsToGitRange verifies that calling runCommitlint
// with no explicit input (no args, no --from, no --stdin) falls back to linting
// commits since the last git tag — the documented default behaviour.
// It must not panic and may return nil or a non-nil error depending on whether a
// valid git repository with tags is present in the working directory.
func TestRunCommitlint_NoArgs_DefaultsToGitRange(t *testing.T) {
	err := runCommitlint(context.TODO(), []string{}, "", "HEAD", false, "text")
	// Any outcome (nil or error) is acceptable; the important invariant is no panic.
	t.Logf("runCommitlint (no args) returned: %v", err)
}

func TestCommitlintSummary_Fields(t *testing.T) {
	s := CommitlintSummary{
		Valid:  false,
		Total:  3,
		Passed: 2,
		Failed: 1,
		Results: []CommitlintResult{
			{Message: "feat: ok", Valid: true},
			{Message: "fix: ok", Valid: true},
			{Message: "bad", Valid: false, Error: "not a Conventional Commit"},
		},
	}
	if s.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", s.Failed)
	}
	if s.Passed != 2 {
		t.Errorf("expected Passed=2, got %d", s.Passed)
	}
	if len(s.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(s.Results))
	}
}

func TestNewReleaseCommand_EditFlag(t *testing.T) {
	cmd := NewRootCommand()
	for _, sub := range cmd.Commands() {
		if sub.Use == "release" {
			if f := sub.Flags().Lookup("edit"); f == nil {
				t.Error("expected --edit flag on release command")
			}
			return
		}
	}
	t.Error("release command not found")
}

func TestPluginSpecsFromConfig(t *testing.T) {
	plugins := []config.PluginConfig{{Uses: "github", Args: map[string]interface{}{"token": "x"}}}
	specs := pluginSpecsFromConfig(plugins)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Uses != "github" {
		t.Fatalf("unexpected uses value %q", specs[0].Uses)
	}
	if specs[0].Config["token"] != "x" {
		t.Fatalf("unexpected config %#v", specs[0].Config)
	}
}

func TestPluginBinaryName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SemRels/github@1.2.3", "semrel-plugin-github"},
		{"github", "semrel-plugin-github"},
		{"provider-github", "semrel-plugin-github"}, // category prefix stripped
		{"github-actions", "semrel-plugin-github-actions"},
		{"condition-github-actions", "semrel-plugin-github-actions"}, // prefix stripped
		{"condition-github-actions@0.1.0", "semrel-plugin-github-actions"},
		{"updater-go", "semrel-plugin-go"},
		{"go", "semrel-plugin-go"},
		{"semrel-plugin-github", "semrel-plugin-github"}, // idempotent
	}
	for _, tt := range tests {
		got := pluginBinaryName(tt.input)
		if got != tt.want {
			t.Errorf("pluginBinaryName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPluginEnvKey(t *testing.T) {
	if got := pluginEnvKey("webhook-url.value"); got != "WEBHOOK_URL_VALUE" {
		t.Fatalf("pluginEnvKey() = %q", got)
	}
}

func TestResolvePluginBinaryPrefersLocalInstall(t *testing.T) {
	home := t.TempDir()
	local := filepath.Join(home, ".semrel", "plugins")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatalf("mkdir local plugin dir: %v", err)
	}

	// Create a platform-appropriate local plugin binary.
	var localPlugin string
	if runtime.GOOS == "windows" {
		localPlugin = filepath.Join(local, "semrel-plugin-demo.cmd")
		if err := os.WriteFile(localPlugin, []byte("@echo off\r\n"), 0o644); err != nil {
			t.Fatalf("write local plugin: %v", err)
		}
		t.Setenv("USERPROFILE", home)
		t.Setenv("PATHEXT", ".CMD;.EXE")
	} else {
		localPlugin = filepath.Join(local, "semrel-plugin-demo")
		if err := os.WriteFile(localPlugin, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write local plugin: %v", err)
		}
		t.Setenv("HOME", home)
	}

	resolved, err := resolvePluginBinary(plugininstance.PluginSpec{Uses: "demo"})
	if err != nil {
		t.Fatalf("resolvePluginBinary() error = %v", err)
	}
	if resolved != localPlugin {
		t.Fatalf("resolvePluginBinary() = %q, want %q", resolved, localPlugin)
	}
}

func TestMakePluginRunnerMissingPluginIsNonFatal(t *testing.T) {
	// Point auto-install at a local server that has no matching plugin so the test
	// does not make real network calls and completes quickly.
	t.Setenv(registry.EnvRegistryURL, "http://127.0.0.1:0") // connection refused → fast fail
	t.Setenv(registry.EnvCacheDir, t.TempDir())
	runner := makePluginRunner(false, ReleaseSummary{})
	if err := runner(context.Background(), plugininstance.PluginSpec{Uses: "definitely-not-installed"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
