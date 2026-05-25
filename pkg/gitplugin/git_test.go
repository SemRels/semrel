// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package gitplugin_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/gitplugin"
)

// runGit runs a git command in dir and logs errors.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// initGitRepo creates a minimal git repo for testing.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")

	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("# Test"), 0o644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-m", "init")
	return dir
}

func TestExpandTagName(t *testing.T) {
	tests := []struct {
		template string
		version  string
		expected string
	}{
		{"v{version}", "1.2.3", "v1.2.3"},
		{"{version}", "2.0.0", "2.0.0"},
		{"myapp/v{version}", "1.0.0", "myapp/v1.0.0"},
		{"static-tag", "1.0.0", "static-tag"},
	}

	for _, tt := range tests {
		got := gitplugin.ExpandTagName(tt.template, tt.version)
		if got != tt.expected {
			t.Errorf("ExpandTagName(%q, %q) = %q, want %q", tt.template, tt.version, got, tt.expected)
		}
	}
}

func TestNewPlugin_Defaults(t *testing.T) {
	p := gitplugin.NewPlugin(gitplugin.Config{TagName: "v{version}"})
	if p.Name() != "git" {
		t.Errorf("expected name 'git', got %q", p.Name())
	}
	if p.Version() == "" {
		t.Error("expected non-empty version")
	}
}

func TestPlugin_Validate_MissingTag(t *testing.T) {
	p := gitplugin.NewPlugin(gitplugin.Config{})
	if err := p.Validate(); err == nil {
		t.Error("expected error for missing tag_name")
	}
}

func TestPlugin_Validate_OK(t *testing.T) {
	p := gitplugin.NewPlugin(gitplugin.Config{TagName: "v{version}"})
	if err := p.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPlugin_CreateTag(t *testing.T) {
	dir := initGitRepo(t)
	p := gitplugin.NewPlugin(gitplugin.Config{TagName: "v{version}", Remote: "origin"})

	if err := p.CreateTag(context.Background(), dir, "1.2.3"); err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}

	// Verify tag was created
	out := runGit(t, dir, "tag", "-l", "v1.2.3")
	if !strings.Contains(out, "v1.2.3") {
		t.Errorf("expected tag v1.2.3 to exist, got: %q", out)
	}
}

func TestPlugin_CommitFiles_Empty(t *testing.T) {
	p := gitplugin.NewPlugin(gitplugin.Config{TagName: "v{version}", Files: nil})
	// No files to commit — should be a no-op
	if err := p.CommitFiles(context.Background(), ".", "1.0.0"); err != nil {
		t.Errorf("unexpected error with empty files: %v", err)
	}
}

func TestPlugin_CommitFiles_WithFiles(t *testing.T) {
	dir := initGitRepo(t)

	// Write a file to commit
	os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("## v1.0.0\n- feature"), 0o644)

	p := gitplugin.NewPlugin(gitplugin.Config{
		TagName:       "v{version}",
		CommitMessage: "chore: release {version}",
		Files:         []string{"CHANGELOG.md"},
		Remote:        "origin",
	})

	if err := p.CommitFiles(context.Background(), dir, "1.0.0"); err != nil {
		t.Fatalf("CommitFiles failed: %v", err)
	}

	// Verify commit was created
	out := runGit(t, dir, "log", "--oneline", "-1")
	if !strings.Contains(out, "release 1.0.0") {
		t.Errorf("expected commit message to contain 'release 1.0.0', got: %q", out)
	}
}
