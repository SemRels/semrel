// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package colors

import (
	"strings"
	"testing"
)

func TestColorsEnabled(t *testing.T) {
	// Force enable for testing
	Enable()
	if !IsEnabled() {
		t.Error("expected enabled after Enable()")
	}
	if !strings.Contains(Bold("x"), "\033[1m") {
		t.Error("expected bold escape in output")
	}
	if !strings.Contains(Green("x"), "\033[32m") {
		t.Error("expected green escape in output")
	}
	if !strings.Contains(Red("x"), "\033[31m") {
		t.Error("expected red escape in output")
	}
}

func TestColorsDisabled(t *testing.T) {
	Disable()
	defer Enable() // restore for other tests
	if IsEnabled() {
		t.Error("expected disabled after Disable()")
	}
	if Bold("hello") != "hello" {
		t.Error("expected plain text when disabled")
	}
	if Green("world") != "world" {
		t.Error("expected plain text when disabled")
	}
}

func TestSuccessWarningError(t *testing.T) {
	Enable()
	if !strings.Contains(Success("ok"), "✓ ok") {
		t.Error("Success missing checkmark")
	}
	if !strings.Contains(Warning("warn"), "⚠ warn") {
		t.Error("Warning missing symbol")
	}
	if !strings.Contains(Error("fail"), "✗ fail") {
		t.Error("Error missing symbol")
	}
}

func TestSuccessWarningError_Disabled(t *testing.T) {
	Disable()
	defer Enable()
	if Success("ok") != "✓ ok" {
		t.Errorf("unexpected Success output: %q", Success("ok"))
	}
	if Warning("w") != "⚠ w" {
		t.Errorf("unexpected Warning output: %q", Warning("w"))
	}
	if Error("e") != "✗ e" {
		t.Errorf("unexpected Error output: %q", Error("e"))
	}
}
