// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package releasenotes

import (
	"strings"
	"testing"
	"time"

	"github.com/GoSemantics/semrel/pkg/commits"
)

func TestBuild(t *testing.T) {
	cs := []*commits.Commit{
		{Type: "feat", Scope: "auth", Description: "add login"},
		{Type: "fix", Description: "fix crash"},
		{Type: "feat", IsBreakingChange: true, Description: "redesign API"},
		{Type: "chore", Description: "update deps"},
		nil,
	}
	rn := Build("v1.0.0", cs)
	if rn.Version != "v1.0.0" {
		t.Errorf("Version: %q", rn.Version)
	}
	if len(rn.Breaking) != 1 {
		t.Errorf("Breaking: got %d want 1", len(rn.Breaking))
	}
	if len(rn.Features) != 1 {
		t.Errorf("Features: got %d want 1", len(rn.Features))
	}
	if len(rn.Fixes) != 1 {
		t.Errorf("Fixes: got %d want 1", len(rn.Fixes))
	}
	if len(rn.Others) != 1 {
		t.Errorf("Others: got %d want 1", len(rn.Others))
	}
}

func TestBumpLevel(t *testing.T) {
	tests := []struct {
		name string
		rn   ReleaseNotes
		want string
	}{
		{"empty", ReleaseNotes{}, ""},
		{"patch only", ReleaseNotes{Fixes: []Entry{{}}}, "patch"},
		{"minor", ReleaseNotes{Features: []Entry{{}}}, "minor"},
		{"major", ReleaseNotes{Breaking: []Entry{{}}}, "major"},
		{"major overrides minor", ReleaseNotes{Features: []Entry{{}}, Breaking: []Entry{{}}}, "major"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rn.BumpLevel(); got != tt.want {
				t.Errorf("BumpLevel() = %q want %q", got, tt.want)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	if !(ReleaseNotes{}).IsEmpty() {
		t.Error("empty ReleaseNotes should be empty")
	}
	rn := ReleaseNotes{Features: []Entry{{}}}
	if rn.IsEmpty() {
		t.Error("non-empty ReleaseNotes should not be empty")
	}
}

func TestRenderMarkdown(t *testing.T) {
	rn := &ReleaseNotes{
		Version: "v1.0.0",
		Date:    time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Breaking: []Entry{{Scope: "api", Description: "breaking change"}},
		Features: []Entry{{Description: "add feature"}},
		Fixes:    []Entry{{Scope: "db", Description: "fix query"}},
		Others:   []Entry{{Description: "update ci"}},
	}
	md := rn.RenderMarkdown()
	for _, want := range []string{
		"## v1.0.0 (2026-05-25)",
		"### ⚠ BREAKING CHANGES",
		"**api:** breaking change",
		"### Features",
		"add feature",
		"### Bug Fixes",
		"**db:** fix query",
		"### Other Changes",
		"update ci",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
}

func TestRenderText(t *testing.T) {
	rn := &ReleaseNotes{
		Version:  "v1.0.0",
		Date:     time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Breaking: []Entry{{}},
		Features: []Entry{{}, {}},
		Fixes:    []Entry{{}},
	}
	text := rn.RenderText()
	if !strings.Contains(text, "v1.0.0") {
		t.Error("missing version")
	}
	if !strings.Contains(text, "1 breaking") {
		t.Error("missing breaking count")
	}
	if !strings.Contains(text, "2 new feature") {
		t.Error("missing feature count")
	}
}

func TestRenderMarkdown_EmptyDate(t *testing.T) {
	rn := &ReleaseNotes{
		Version:  "v1.0.0",
		Features: []Entry{{Description: "something"}},
	}
	md := rn.RenderMarkdown()
	if !strings.Contains(md, "## v1.0.0") {
		t.Error("missing version header")
	}
}
