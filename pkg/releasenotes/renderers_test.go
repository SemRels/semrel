// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package releasenotes

import (
	"strings"
	"testing"
	"time"
)

func TestRenderArtifactHub_Basic(t *testing.T) {
	rn := &ReleaseNotes{
		Version: "v1.2.0",
		Date:    time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Features: []Entry{
			{Type: "feat", Description: "add webhook support"},
			{Type: "feat", Scope: "api", Description: "new auth endpoint"},
		},
		Fixes: []Entry{
			{Type: "fix", Description: "fix null pointer"},
		},
		Breaking: []Entry{
			{Type: "feat", Description: "remove legacy auth", IsBreaking: true},
		},
	}
	out := rn.RenderArtifactHub()

	if !strings.Contains(out, "kind: added") {
		t.Error("expected 'kind: added' for features")
	}
	if !strings.Contains(out, "kind: fixed") {
		t.Error("expected 'kind: fixed' for fixes")
	}
	if !strings.Contains(out, "kind: security") {
		t.Error("expected 'kind: security' for breaking changes")
	}
	if !strings.Contains(out, "description: add webhook support") {
		t.Error("missing feature description")
	}
	if !strings.Contains(out, "api: new auth endpoint") {
		t.Error("missing scoped feature description")
	}
}

func TestRenderArtifactHub_Empty(t *testing.T) {
	rn := &ReleaseNotes{Version: "v1.0.0"}
	out := rn.RenderArtifactHub()
	if out != "" {
		t.Errorf("expected empty output for empty release notes, got %q", out)
	}
}

func TestRenderArtifactHub_Others(t *testing.T) {
	rn := &ReleaseNotes{
		Version: "v1.0.0",
		Others:  []Entry{{Type: "chore", Description: "update deps"}},
	}
	out := rn.RenderArtifactHub()
	if !strings.Contains(out, "kind: changed") {
		t.Error("expected 'kind: changed' for chore")
	}
}

func TestRenderTemplate_Basic(t *testing.T) {
	rn := &ReleaseNotes{
		Version:  "v1.2.0",
		Date:     time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Features: []Entry{{Type: "feat", Description: "add webhook"}},
		Fixes:    []Entry{{Type: "fix", Description: "fix crash"}},
	}
	tmpl := `## {{.Version}} ({{.Date}})
{{range .Features}}- feat: {{.Description}}
{{end}}{{range .Fixes}}- fix: {{.Description}}
{{end}}`
	out, err := rn.RenderTemplate(tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "## v1.2.0 (2026-05-25)") {
		t.Errorf("missing header in:\n%s", out)
	}
	if !strings.Contains(out, "- feat: add webhook") {
		t.Error("missing feature line")
	}
	if !strings.Contains(out, "- fix: fix crash") {
		t.Error("missing fix line")
	}
}

func TestRenderTemplate_FuncMap(t *testing.T) {
	rn := &ReleaseNotes{
		Version:  "v1.0.0",
		Features: []Entry{{Type: "feat", Description: "new feature"}},
	}
	tmpl := `{{range .Features}}{{upper .Description}}{{end}}`
	out, err := rn.RenderTemplate(tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "NEW FEATURE" {
		t.Errorf("expected 'NEW FEATURE', got %q", out)
	}
}

func TestRenderTemplate_Truncate(t *testing.T) {
	rn := &ReleaseNotes{
		Version: "v1.0.0",
		Others:  []Entry{{Type: "chore", Description: "a long description that should be cut"}},
	}
	tmpl := `{{range .Others}}{{truncate 10 .Description}}{{end}}`
	out, err := rn.RenderTemplate(tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(out, "a long des") {
		t.Errorf("unexpected truncation result: %q", out)
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	rn := &ReleaseNotes{Version: "v1.0.0"}
	_, err := rn.RenderTemplate("{{.Invalid.Deeply.Nested}")
	if err == nil {
		t.Error("expected error for invalid template")
	}
}
