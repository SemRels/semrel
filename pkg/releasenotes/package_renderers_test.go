// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package releasenotes

import (
	"strings"
	"testing"
)

func sampleNotes() *ReleaseNotes {
	return &ReleaseNotes{
		Breaking: []Entry{
			{Type: "feat", Scope: "api", Description: "remove legacy endpoint", IsBreaking: true},
		},
		Features: []Entry{
			{Type: "feat", Scope: "auth", Description: "support OAuth2 PKCE"},
			{Type: "feat", Scope: "", Description: "add dark mode"},
		},
		Fixes: []Entry{
			{Type: "fix", Scope: "", Description: "prevent nil dereference on empty input"},
		},
	}
}

// ---------------------------------------------------------------------------
// NuGet
// ---------------------------------------------------------------------------

func TestRenderNuGetReleaseNotes_Full(t *testing.T) {
	notes := sampleNotes()
	out := notes.RenderNuGetReleaseNotes("v1.2.0")

	if !strings.Contains(out, "v1.2.0") {
		t.Error("expected version header")
	}
	if !strings.Contains(out, "Breaking Changes:") {
		t.Error("expected Breaking Changes section")
	}
	if !strings.Contains(out, "(api) remove legacy endpoint") {
		t.Error("expected breaking entry with scope")
	}
	if !strings.Contains(out, "Features:") {
		t.Error("expected Features section")
	}
	if !strings.Contains(out, "(auth) support OAuth2 PKCE") {
		t.Error("expected feature with scope")
	}
	if !strings.Contains(out, "add dark mode") {
		t.Error("expected feature without scope")
	}
	if !strings.Contains(out, "Bug Fixes:") {
		t.Error("expected Bug Fixes section")
	}
	if !strings.Contains(out, "prevent nil dereference") {
		t.Error("expected fix entry")
	}
}

func TestRenderNuGetReleaseNotes_Empty(t *testing.T) {
	notes := &ReleaseNotes{}
	out := notes.RenderNuGetReleaseNotes("v1.0.0")
	if out != "v1.0.0" {
		t.Errorf("expected just version header for empty notes, got %q", out)
	}
}

func TestRenderNuGetReleaseNotes_NoVersion(t *testing.T) {
	notes := &ReleaseNotes{
		Features: []Entry{{Type: "feat", Description: "add feature"}},
	}
	out := notes.RenderNuGetReleaseNotes("")
	if strings.HasPrefix(out, "\n") {
		t.Error("should not start with newline when no version")
	}
	if !strings.Contains(out, "Features:") {
		t.Error("expected Features section")
	}
}

// ---------------------------------------------------------------------------
// PyPI / RST
// ---------------------------------------------------------------------------

func TestRenderPyPIChangelog_WithDate(t *testing.T) {
	notes := sampleNotes()
	out := notes.RenderPyPIChangelog("v1.2.0", "2026-01-15")

	if !strings.Contains(out, "v1.2.0 (2026-01-15)") {
		t.Error("expected versioned heading with date")
	}
	// RST heading underline
	if !strings.Contains(out, "---") {
		t.Error("expected RST underline")
	}
	if !strings.Contains(out, "**Breaking Changes**") {
		t.Error("expected Breaking Changes RST section")
	}
	if !strings.Contains(out, "``api``: remove legacy endpoint") {
		t.Error("expected RST inline code scope")
	}
	if !strings.Contains(out, "**New Features**") {
		t.Error("expected New Features section")
	}
	if !strings.Contains(out, "``auth``: support OAuth2 PKCE") {
		t.Error("expected feature with RST scope")
	}
	if !strings.Contains(out, "**Bug Fixes**") {
		t.Error("expected Bug Fixes section")
	}
}

func TestRenderPyPIChangelog_NoDate(t *testing.T) {
	notes := &ReleaseNotes{
		Features: []Entry{{Type: "feat", Description: "add feature"}},
	}
	out := notes.RenderPyPIChangelog("v1.0.0", "")
	if !strings.Contains(out, "v1.0.0\n") {
		t.Errorf("expected simple version heading, got %q", out)
	}
	if strings.Contains(out, "(") {
		t.Error("should not have date parentheses when date is empty")
	}
}

func TestRenderPyPIChangelog_EntryWithoutScope(t *testing.T) {
	notes := &ReleaseNotes{
		Features: []Entry{{Type: "feat", Description: "add feature"}},
	}
	out := notes.RenderPyPIChangelog("v1.0.0", "")
	if strings.Contains(out, "``") {
		t.Error("should not have RST inline code when scope is empty")
	}
	if !strings.Contains(out, "- add feature") {
		t.Error("expected plain bullet")
	}
}

// ---------------------------------------------------------------------------
// Maven / Gradle
// ---------------------------------------------------------------------------

func TestRenderMavenChangelog_Full(t *testing.T) {
	notes := sampleNotes()
	out := notes.RenderMavenChangelog("v1.2.0")

	if !strings.Contains(out, "## v1.2.0") {
		t.Error("expected Markdown h2 heading")
	}
	if !strings.Contains(out, "### ⚠ Breaking Changes") {
		t.Error("expected Breaking Changes section")
	}
	if !strings.Contains(out, "**api:** remove legacy endpoint") {
		t.Error("expected breaking entry with bold scope")
	}
	if !strings.Contains(out, "### ✨ Features") {
		t.Error("expected Features section with emoji")
	}
	if !strings.Contains(out, "**auth:** support OAuth2 PKCE") {
		t.Error("expected feature with scope")
	}
	if !strings.Contains(out, "### 🐛 Bug Fixes") {
		t.Error("expected Bug Fixes section with emoji")
	}
	if !strings.Contains(out, "- prevent nil dereference") {
		t.Error("expected fix without scope")
	}
}

func TestRenderMavenChangelog_Empty(t *testing.T) {
	notes := &ReleaseNotes{}
	out := notes.RenderMavenChangelog("v1.0.0")
	if out != "## v1.0.0" {
		t.Errorf("expected only heading for empty notes, got %q", out)
	}
}

func TestRenderMavenChangelog_NoVersion(t *testing.T) {
	notes := &ReleaseNotes{
		Fixes: []Entry{{Type: "fix", Description: "fix crash"}},
	}
	out := notes.RenderMavenChangelog("")
	if strings.HasPrefix(out, "## ") {
		t.Error("should not have heading with empty version")
	}
	if !strings.Contains(out, "fix crash") {
		t.Error("expected fix entry")
	}
}
