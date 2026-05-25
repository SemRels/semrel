// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package editor

import (
	"os"
	"strings"
	"testing"
)

func TestNew_ReturnsEditor(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("New() returned nil")
	}
	if e.Command == "" {
		t.Error("expected non-empty Command from New()")
	}
}

func TestNewWithCommand(t *testing.T) {
	e := NewWithCommand("myeditor --flag")
	if e.Command != "myeditor --flag" {
		t.Errorf("expected 'myeditor --flag', got %q", e.Command)
	}
}

func TestEditor_Edit_UsesScriptAsEditor(t *testing.T) {
	// Use a script that appends a line to the file as a mock editor.
	// On both Unix and Windows "echo" is available.
	// Strategy: use a shell one-liner via a temp script.

	// Write a temp script that appends "EDITED" to the file
	scriptContent := "#!/bin/sh\necho EDITED >> \"$1\"\n"
	scriptFile, err := os.CreateTemp("", "mock-editor-*.sh")
	if err != nil {
		t.Skip("cannot create temp script: " + err.Error())
	}
	defer os.Remove(scriptFile.Name())
	scriptFile.WriteString(scriptContent)
	scriptFile.Close()
	os.Chmod(scriptFile.Name(), 0o755)

	// On Windows, skip this sh-based test
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("skipping sh-based mock editor test on this platform")
	}

	e := NewWithCommand("sh " + scriptFile.Name())
	result, err := e.Edit("original content\n")
	if err != nil {
		t.Fatalf("Edit() error: %v", err)
	}
	if !strings.Contains(result, "original content") {
		t.Error("expected original content to be preserved")
	}
	if !strings.Contains(result, "EDITED") {
		t.Error("expected EDITED appended by mock editor")
	}
}

func TestEditor_Edit_InvalidCommand(t *testing.T) {
	e := NewWithCommand("/nonexistent/editor/binary")
	_, err := e.Edit("some content")
	if err == nil {
		t.Fatal("expected error for invalid editor command")
	}
}

func TestEditor_Edit_EmptyCommand(t *testing.T) {
	e := NewWithCommand("")
	_, err := e.Edit("some content")
	if err == nil {
		t.Fatal("expected error for empty editor command")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got %q", err.Error())
	}
}

func TestResolveEditor_VisualEnv(t *testing.T) {
	old := os.Getenv("VISUAL")
	defer os.Setenv("VISUAL", old)

	os.Setenv("VISUAL", "my-visual-editor")
	got := resolveEditor()
	if got != "my-visual-editor" {
		t.Errorf("expected VISUAL env to be used, got %q", got)
	}
}

func TestResolveEditor_EditorEnvFallback(t *testing.T) {
	oldVisual := os.Getenv("VISUAL")
	oldEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", oldVisual)
		os.Setenv("EDITOR", oldEditor)
	}()

	os.Unsetenv("VISUAL")
	os.Setenv("EDITOR", "my-editor")
	got := resolveEditor()
	if got != "my-editor" {
		t.Errorf("expected EDITOR env to be used, got %q", got)
	}
}
