// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package commits

import "testing"

func TestParse_ConventionalCommit(t *testing.T) {
	p := NewParser()
	tests := []struct {
		name         string
		msg          string
		wantType     string
		wantScope    string
		wantDesc     string
		wantBreaking bool
	}{
		{
			name:     "feat with scope",
			msg:      "feat(auth): add OAuth2 support",
			wantType: "feat", wantScope: "auth", wantDesc: "add OAuth2 support",
		},
		{
			name:     "fix without scope",
			msg:      "fix: correct off-by-one error",
			wantType: "fix", wantDesc: "correct off-by-one error",
		},
		{
			name:         "breaking change with !",
			msg:          "feat!: drop support for Node 6",
			wantType:     "feat",
			wantDesc:     "drop support for Node 6",
			wantBreaking: true,
		},
		{
			name:         "breaking change with scope and !",
			msg:          "chore(api)!: remove deprecated endpoints",
			wantType:     "chore",
			wantScope:    "api",
			wantDesc:     "remove deprecated endpoints",
			wantBreaking: true,
		},
		{
			name:         "breaking change in body",
			msg:          "feat: new config format\n\nBREAKING CHANGE: config file renamed",
			wantType:     "feat",
			wantBreaking: true,
		},
		{
			name:     "non-conventional commit",
			msg:      "Merge branch 'feature/x' into main",
			wantType: "", wantDesc: "Merge branch 'feature/x' into main",
		},
		{
			name:     "chore type",
			msg:      "chore: update dependencies",
			wantType: "chore", wantDesc: "update dependencies",
		},
		{
			name:     "docs type with scope",
			msg:      "docs(readme): add installation guide",
			wantType: "docs", wantScope: "readme", wantDesc: "add installation guide",
		},
		{
			name:     "ci type",
			msg:      "ci: fix GitHub Actions workflow",
			wantType: "ci", wantDesc: "fix GitHub Actions workflow",
		},
		{
			name:     "refactor type",
			msg:      "refactor(core): extract config loader",
			wantType: "refactor", wantScope: "core", wantDesc: "extract config loader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := p.Parse(tt.msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.Type != tt.wantType {
				t.Errorf("Type: got %q want %q", c.Type, tt.wantType)
			}
			if c.Scope != tt.wantScope {
				t.Errorf("Scope: got %q want %q", c.Scope, tt.wantScope)
			}
			if tt.wantDesc != "" && c.Description != tt.wantDesc {
				t.Errorf("Description: got %q want %q", c.Description, tt.wantDesc)
			}
			if c.IsBreakingChange != tt.wantBreaking {
				t.Errorf("IsBreakingChange: got %v want %v", c.IsBreakingChange, tt.wantBreaking)
			}
		})
	}
}

func TestParse_EmptyMessage(t *testing.T) {
	p := NewParser()
	_, err := p.Parse("")
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestParse_RawPreserved(t *testing.T) {
	p := NewParser()
	msg := "feat: some feature\n\nLong body here."
	c, err := p.Parse(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Raw != msg {
		t.Errorf("Raw not preserved: got %q want %q", c.Raw, msg)
	}
}

func TestParse_BodyPreserved(t *testing.T) {
	p := NewParser()
	msg := "fix: patch something\n\nThis is the body."
	c, err := p.Parse(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Body != "This is the body." {
		t.Errorf("Body: got %q", c.Body)
	}
}

func TestParseAll(t *testing.T) {
	p := NewParser()
	msgs := []string{
		"feat: add feature",
		"fix: fix bug",
		"",
	}
	cs := p.ParseAll(msgs)
	// empty string is skipped (returns error), but non-conventional ones are kept
	if len(cs) != 2 {
		t.Errorf("expected 2 commits, got %d", len(cs))
	}
}

func TestParseMulti_MultipleTypes(t *testing.T) {
	p := NewParser()
	msg := "feat(api): add webhook support\nfix(api): validate empty payload\nchore: update deps"
	cs, err := p.ParseMulti(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// feat should be first (highest priority)
	if len(cs) == 0 {
		t.Fatal("expected commits")
	}
	if cs[0].Type != "feat" {
		t.Errorf("first commit should be feat, got %q", cs[0].Type)
	}
	// Should have all three types
	types := make(map[string]bool)
	for _, c := range cs {
		types[c.Type] = true
	}
	if !types["feat"] || !types["fix"] || !types["chore"] {
		t.Errorf("missing types: %v", types)
	}
}

func TestParseMulti_BreakingFirst(t *testing.T) {
	p := NewParser()
	msg := "fix: small fix\nfeat!: breaking feature"
	cs, err := p.ParseMulti(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) == 0 {
		t.Fatal("expected commits")
	}
	if !cs[0].IsBreakingChange {
		t.Errorf("breaking commit should be first, got type=%q breaking=%v", cs[0].Type, cs[0].IsBreakingChange)
	}
}

func TestParseMulti_FallbackSingleCommit(t *testing.T) {
	p := NewParser()
	msg := "feat: single conventional commit"
	cs, err := p.ParseMulti(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(cs))
	}
	if cs[0].Type != "feat" {
		t.Errorf("got type %q want feat", cs[0].Type)
	}
}

func TestParseMulti_FeatPlusFix_ResultsInMinor(t *testing.T) {
	p := NewParser()
	// feat+fix in one commit should yield highest = minor (feat)
	msg := "feat: add new endpoint\nfix: correct validation"
	cs, err := p.ParseMulti(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cs[0].Type != "feat" {
		t.Errorf("expected feat first, got %q", cs[0].Type)
	}
}

func TestParseMulti_EmptyMessage(t *testing.T) {
	p := NewParser()
	_, err := p.ParseMulti("")
	if err == nil {
		t.Error("expected error for empty message")
	}
}
