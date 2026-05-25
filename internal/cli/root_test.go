// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package cli provides placeholder tests for the root command.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/config"
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
	}
}

func TestIsBranchConfigured(t *testing.T) {
	tests := []struct {
		branch   string
		branches []config.BranchConfig
		want     bool
	}{
		{"main", nil, true},                                              // no config = all branches
		{"main", []config.BranchConfig{{Name: "main"}}, true},           // exact match
		{"main", []config.BranchConfig{{Name: "develop"}}, false},       // not in list
		{"next", []config.BranchConfig{{Name: "main"}, {Name: "next"}}, true}, // second match
		{"feature/x", []config.BranchConfig{{Name: "main"}}, false},    // feature branch not listed
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

