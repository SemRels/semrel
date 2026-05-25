// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/commits"
)

func TestWriteFile(t *testing.T) {
	gen := NewGenerator()
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	cs := []*commits.Commit{
		{Type: "feat", Description: "add feature"},
	}

	if err := gen.WriteFile(path, "v1.0.0", cs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "## v1.0.0") {
		t.Error("file should contain version header")
	}
	if !strings.Contains(string(data), "add feature") {
		t.Error("file should contain commit description")
	}
}

func TestPrependToFile_NewFile(t *testing.T) {
	gen := NewGenerator()
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	cs := []*commits.Commit{
		{Type: "fix", Description: "fix bug"},
	}

	if err := gen.PrependToFile(path, "v1.0.1", cs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# Changelog") {
		t.Error("file should have Changelog header")
	}
	if !strings.Contains(string(data), "v1.0.1") {
		t.Error("file should contain version")
	}
}

func TestPrependToFile_ExistingFile(t *testing.T) {
	gen := NewGenerator()
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	// Write initial entry
	cs1 := []*commits.Commit{{Type: "feat", Description: "feature A"}}
	gen.PrependToFile(path, "v1.0.0", cs1)

	// Prepend second entry
	cs2 := []*commits.Commit{{Type: "feat", Description: "feature B"}}
	if err := gen.PrependToFile(path, "v1.1.0", cs2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	// v1.1.0 should appear before v1.0.0
	idxNew := strings.Index(content, "v1.1.0")
	idxOld := strings.Index(content, "v1.0.0")
	if idxNew == -1 || idxOld == -1 {
		t.Errorf("both versions should be present, got:\n%s", content)
	}
	if idxNew > idxOld {
		t.Error("newer version should appear before older version")
	}
}
