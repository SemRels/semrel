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
	if strings.Contains(output, "###") {
		t.Error("empty changelog should have no section headers")
	}
}

func TestGenerate_NilCommitsFiltered(t *testing.T) {
	gen := NewGenerator()
	cs := []*commits.Commit{nil, {Type: "fix", Description: "fix thing"}, nil}
	output := gen.Generate("v1.0.1", cs)
	if !strings.Contains(output, "fix thing") {
		t.Error("valid commit missing from output")
	}
}

func TestGenerate_PerfAndRevertInBugFixes(t *testing.T) {
	gen := NewGenerator()
	cs := []*commits.Commit{
		{Type: "perf", Description: "speed up query"},
		{Type: "revert", Description: "revert commit abc"},
	}
	output := gen.Generate("v1.0.2", cs)
	if !strings.Contains(output, "### Bug Fixes") {
		t.Error("perf/revert should appear in Bug Fixes")
	}
	if !strings.Contains(output, "speed up query") {
		t.Error("missing perf commit")
	}
	if !strings.Contains(output, "revert commit abc") {
		t.Error("missing revert commit")
	}
}

func TestGenerate_OtherChangesSection(t *testing.T) {
	gen := NewGenerator()
	cs := []*commits.Commit{
		{Type: "chore", Description: "update deps"},
		{Type: "docs", Description: "update README"},
		{Type: "ci", Description: "fix workflow"},
	}
	output := gen.Generate("v1.0.3", cs)
	if !strings.Contains(output, "### Other Changes") {
		t.Error("non-standard types should appear in Other Changes")
	}
}

func TestGenerate_NoScope(t *testing.T) {
	gen := NewGenerator()
	cs := []*commits.Commit{
		{Type: "feat", Description: "add feature without scope"},
	}
	output := gen.Generate("v1.0.0", cs)
	if strings.Contains(output, "**:**") {
		t.Error("empty scope should not be rendered as **:**")
	}
	if !strings.Contains(output, "add feature without scope") {
		t.Error("description missing from output")
	}
}

func TestGenerate_BreakingWithoutType(t *testing.T) {
	gen := NewGenerator()
	cs := []*commits.Commit{
		{IsBreakingChange: true, Description: "breaking with no type"},
	}
	output := gen.Generate("v2.0.0", cs)
	if !strings.Contains(output, "### ⚠ BREAKING CHANGES") {
		t.Error("breaking change without type should appear in breaking section")
	}
}
