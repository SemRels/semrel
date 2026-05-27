// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package editor provides an interactive release notes editor for semrel.
// When --edit is passed to `semrel release`, the generated release notes are
// opened in the user's preferred editor ($VISUAL or $EDITOR, defaulting to
// vi on Unix and notepad on Windows) before the release is finalised.
//
// The flow:
//  1. semrel generates the draft release notes
//  2. Notes are written to a temp file
//  3. The editor is launched with the temp file
//  4. After the editor exits, the edited content is read back
//  5. The edited content replaces the original for the release
//
// This allows operators to add a manual summary, fix typos, or redact
// sensitive information from the auto-generated changelog.
//
// See: https://github.com/SemRels/semrel/issues/48
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DefaultEditors lists the editors tried in order when neither $VISUAL nor
// $EDITOR is set.
var DefaultEditors = []string{"vi", "nano", "notepad"}

// Editor opens a text editor for the user to edit content.
type Editor struct {
	// Command is the editor binary path or name.
	// If empty, it is resolved from $VISUAL, $EDITOR, or DefaultEditors.
	Command string
}

// New returns an Editor with the command resolved from the environment.
func New() *Editor {
	return &Editor{Command: resolveEditor()}
}

// NewWithCommand returns an Editor using the given command string.
func NewWithCommand(cmd string) *Editor {
	return &Editor{Command: cmd}
}

// Edit opens the given content in the editor and returns the edited content.
// A temporary file is created, the editor is launched, and the file is read
// back after the editor exits. The temporary file is always cleaned up.
func (e *Editor) Edit(content string) (string, error) {
	f, err := os.CreateTemp("", "semrel-notes-*.md")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(f.Name()) //nolint:errcheck

	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("writing to temp file: %w", err)
	}
	_ = f.Close()

	if err := e.launch(f.Name()); err != nil {
		return "", fmt.Errorf("launching editor: %w", err)
	}

	data, err := os.ReadFile(f.Name())
	if err != nil {
		return "", fmt.Errorf("reading edited file: %w", err)
	}
	return string(data), nil
}

// launch starts the editor process with the given file and waits for it to exit.
func (e *Editor) launch(path string) error {
	parts := strings.Fields(e.Command)
	if len(parts) == 0 {
		return fmt.Errorf("editor command is empty")
	}
	args := append(parts[1:], path) //nolint:gocritic
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// resolveEditor returns the editor to use, checking $VISUAL and $EDITOR first,
// then falling back to platform defaults.
func resolveEditor() string {
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	// Try common editors in order
	for _, e := range []string{"vi", "nano"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return "vi"
}
