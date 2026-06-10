// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInteractiveConfirm_Yes(t *testing.T) {
	for _, answer := range []string{"y", "Y", "yes", ""} {
		t.Run("answer="+answer, func(t *testing.T) {
			r := strings.NewReader(answer + "\n")
			var w bytes.Buffer
			tag, err := interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n\n### Features\n- test\n", false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tag != "v1.1.0" {
				t.Errorf("expected v1.1.0, got %q", tag)
			}
		})
	}
}

func TestInteractiveConfirm_No(t *testing.T) {
	for _, answer := range []string{"n", "N", "no"} {
		t.Run("answer="+answer, func(t *testing.T) {
			r := strings.NewReader(answer + "\n")
			var w bytes.Buffer
			_, err := interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n", false)
			if err == nil {
				t.Fatal("expected error for 'no' answer")
			}
			if !strings.Contains(err.Error(), "aborted") {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestInteractiveConfirm_Edit(t *testing.T) {
	// First send "edit", then the custom version.
	r := strings.NewReader("edit\nv2.0.0\n")
	var w bytes.Buffer
	tag, err := interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", tag)
	}
}

func TestInteractiveConfirm_EditEmptyThenValue(t *testing.T) {
	// Empty custom version triggers re-prompt, then valid version.
	r := strings.NewReader("edit\n\nedit\nv3.0.0\n")
	var w bytes.Buffer
	tag, err := interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v3.0.0" {
		t.Errorf("expected v3.0.0, got %q", tag)
	}
}

func TestInteractiveConfirm_InvalidThenYes(t *testing.T) {
	// Unknown answer triggers re-prompt.
	r := strings.NewReader("maybe\nY\n")
	var w bytes.Buffer
	tag, err := interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v1.1.0" {
		t.Errorf("expected v1.1.0, got %q", tag)
	}
	if !strings.Contains(w.String(), "Y, n, or edit") {
		t.Errorf("expected hint message in output, got: %s", w.String())
	}
}

func TestInteractiveConfirm_DryRunNote(t *testing.T) {
	r := strings.NewReader("Y\n")
	var w bytes.Buffer
	_, _ = interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n", true)
	if !strings.Contains(w.String(), "dry-run") {
		t.Errorf("expected dry-run note in prompt output, got: %s", w.String())
	}
}

func TestInteractiveConfirm_StdinClosed(t *testing.T) {
	// Empty reader simulates closed stdin.
	r := strings.NewReader("")
	var w bytes.Buffer
	_, err := interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n", false)
	if err == nil {
		t.Fatal("expected error when stdin is closed")
	}
}

func TestInteractiveConfirm_ChangelogPreviewInOutput(t *testing.T) {
	r := strings.NewReader("Y\n")
	var w bytes.Buffer
	_, _ = interactiveConfirm(r, &w, "v1.0.0", "v1.1.0", "minor", "## v1.1.0\n\n### Features\n- new feature\n", false)
	if !strings.Contains(w.String(), "new feature") {
		t.Errorf("expected changelog content in preview, got: %s", w.String())
	}
}
