// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package issues

import (
	"strings"
	"testing"
)

func TestExtract_JiraRef(t *testing.T) {
	e := DefaultExtractor()
	refs := e.Extract("feat(auth): add OAuth2 PROJ-456")
	jira := Filter(refs, "jira")
	if len(jira) != 1 {
		t.Fatalf("expected 1 jira ref, got %d: %v", len(jira), jira)
	}
	if jira[0].ID != "PROJ-456" {
		t.Errorf("expected PROJ-456, got %q", jira[0].ID)
	}
}

func TestExtract_MultipleJira(t *testing.T) {
	e := DefaultExtractor()
	refs := Filter(e.Extract("fix: PROJ-1 and FEAT-999 patched"), "jira")
	if len(refs) != 2 {
		t.Fatalf("expected 2 jira refs, got %d: %v", len(refs), refs)
	}
}

func TestExtract_GitHubRef_Hash(t *testing.T) {
	e := DefaultExtractor()
	refs := Filter(e.Extract("fix: closes #123"), "github")
	if len(refs) != 1 {
		t.Fatalf("expected 1 github ref, got %d: %v", len(refs), refs)
	}
	if refs[0].ID != "123" {
		t.Errorf("expected 123, got %q", refs[0].ID)
	}
}

func TestExtract_GitHubRef_Fixes(t *testing.T) {
	e := DefaultExtractor()
	refs := Filter(e.Extract("fix: fixes #42 and resolves #99"), "github")
	if len(refs) != 2 {
		t.Fatalf("expected 2 github refs, got %d: %v", len(refs), refs)
	}
}

func TestExtract_NoRefs(t *testing.T) {
	e := DefaultExtractor()
	refs := e.Extract("chore: update dependencies")
	if len(refs) != 0 {
		t.Errorf("expected no refs, got %d: %v", len(refs), refs)
	}
}

func TestExtract_Deduplication(t *testing.T) {
	e := DefaultExtractor()
	refs := e.Extract("fix PROJ-1 and also PROJ-1 again")
	jira := Filter(refs, "jira")
	if len(jira) != 1 {
		t.Errorf("expected 1 deduped ref, got %d", len(jira))
	}
}

func TestExtractAll_MultipleMessages(t *testing.T) {
	e := DefaultExtractor()
	msgs := []string{
		"feat: PROJ-100 add feature",
		"fix: PROJ-200 fix bug",
		"chore: nothing here",
		"fix: PROJ-100 same as first", // should be deduped
	}
	refs := Filter(e.ExtractAll(msgs), "jira")
	if len(refs) != 2 {
		t.Errorf("expected 2 unique jira refs, got %d: %v", len(refs), refs)
	}
}

func TestFilter_CaseInsensitive(t *testing.T) {
	refs := []IssueRef{
		{Tracker: "jira", ID: "PROJ-1"},
		{Tracker: "github", ID: "42"},
		{Tracker: "JIRA", ID: "PROJ-2"},
	}
	jira := Filter(refs, "jira")
	if len(jira) != 2 {
		t.Errorf("expected 2 (case-insensitive), got %d", len(jira))
	}
}

func TestIssueRef_Tracker(t *testing.T) {
	e := DefaultExtractor()
	refs := e.Extract("feat: add PROJ-123 fixes #42")
	trackers := make(map[string]bool)
	for _, r := range refs {
		trackers[r.Tracker] = true
	}
	if !trackers["jira"] {
		t.Error("expected jira tracker")
	}
	if !trackers["github"] {
		t.Error("expected github tracker")
	}
	_ = strings.Contains // ensure import used
}
