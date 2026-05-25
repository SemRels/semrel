// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package changelog

import (
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/commits"
)

func TestGenerate(t *testing.T) {
	gen := NewGenerator()
	cs := []*commits.Commit{
		{Type: "feat", Scope: "auth", Description: "add OAuth2"},
		{Type: "fix", Description: "correct typo"},
		{Type: "feat", IsBreakingChange: true, Description: "redesign API"},
	}
	output := gen.Generate("v1.1.0", cs)

	if !strings.Contains(output, "## v1.1.0") {
		t.Error("missing version header")
	}
	if !strings.Contains(output, "### ⚠ BREAKING CHANGES") {
		t.Error("missing breaking changes section")
	}
	if !strings.Contains(output, "### Features") {
		t.Error("missing features section")
	}
	if !strings.Contains(output, "### Bug Fixes") {
		t.Error("missing bug fixes section")
	}
	if !strings.Contains(output, "**auth:** add OAuth2") {
		t.Error("missing scoped commit")
	}
}

func TestGenerate_Empty(t *testing.T) {
	gen := NewGenerator()
	output := gen.Generate("v1.0.0", nil)
	if !strings.Contains(output, "## v1.0.0") {
		t.Error("missing version header in empty changelog")
	}
}
