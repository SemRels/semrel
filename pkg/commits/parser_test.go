// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package commits

import "testing"

func TestParse_ConventionalCommit(t *testing.T) {
	p := NewParser()
	tests := []struct {
		name             string
		msg              string
		wantType         string
		wantScope        string
		wantDesc         string
		wantBreaking     bool
	}{
		{
			name: "feat with scope",
			msg:  "feat(auth): add OAuth2 support",
			wantType: "feat", wantScope: "auth", wantDesc: "add OAuth2 support",
		},
		{
			name: "fix without scope",
			msg:  "fix: correct off-by-one error",
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
			name: "breaking change in body",
			msg:  "feat: new config format\n\nBREAKING CHANGE: config file renamed",
			wantType:     "feat",
			wantBreaking: true,
		},
		{
			name: "non-conventional commit",
			msg:  "Merge branch 'feature/x' into main",
			wantType: "", wantDesc: "Merge branch 'feature/x' into main",
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

func TestParseAll(t *testing.T) {
	p := NewParser()
	msgs := []string{
		"feat: add feature",
		"fix: fix bug",
		"",
	}
	cs := p.ParseAll(msgs)
	if len(cs) != 2 {
		t.Errorf("expected 2 commits, got %d", len(cs))
	}
}
