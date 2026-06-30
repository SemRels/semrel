// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package semver

import (
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantPre   string
		wantMeta  string
		wantErr   bool
	}{
		{input: "1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "v1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "0.0.0", wantMajor: 0, wantMinor: 0, wantPatch: 0},
		{input: "1.2.3-beta.1", wantMajor: 1, wantMinor: 2, wantPatch: 3, wantPre: "beta.1"},
		{input: "1.2.3+build.42", wantMajor: 1, wantMinor: 2, wantPatch: 3, wantMeta: "build.42"},
		{input: "1.2.3-alpha+001", wantMajor: 1, wantMinor: 2, wantPatch: 3, wantPre: "alpha", wantMeta: "001"},
		{input: "", wantMajor: 0, wantMinor: 0, wantPatch: 0},
		{input: "v", wantMajor: 0, wantMinor: 0, wantPatch: 0},
		{input: "not-a-version", wantErr: true},
		{input: "1.2", wantErr: true},
		{input: "1.2.3.4", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseVersion(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseVersion(%q): unexpected error: %v", tt.input, err)
				return
			}
			if v.Major != tt.wantMajor || v.Minor != tt.wantMinor || v.Patch != tt.wantPatch {
				t.Errorf("ParseVersion(%q): got %d.%d.%d want %d.%d.%d",
					tt.input, v.Major, v.Minor, v.Patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
			if v.Prerelease != tt.wantPre {
				t.Errorf("ParseVersion(%q): Prerelease got %q want %q", tt.input, v.Prerelease, tt.wantPre)
			}
			if v.Metadata != tt.wantMeta {
				t.Errorf("ParseVersion(%q): Metadata got %q want %q", tt.input, v.Metadata, tt.wantMeta)
			}
		})
	}
}

func TestNextVersion(t *testing.T) {
	calc := NewCalculator()
	base := &Version{Major: 1, Minor: 2, Patch: 3}

	tests := []struct {
		name        string
		hasFeat     bool
		hasFix      bool
		hasBreaking bool
		wantMajor   int
		wantMinor   int
		wantPatch   int
		wantNil     bool
	}{
		{name: "no changes", wantNil: true},
		{name: "fix bump", hasFix: true, wantMajor: 1, wantMinor: 2, wantPatch: 4},
		{name: "feat bump", hasFeat: true, wantMajor: 1, wantMinor: 3, wantPatch: 0},
		{name: "breaking bump", hasBreaking: true, wantMajor: 2, wantMinor: 0, wantPatch: 0},
		{name: "breaking overrides feat", hasFeat: true, hasBreaking: true, wantMajor: 2, wantMinor: 0, wantPatch: 0},
		{name: "breaking overrides fix", hasFix: true, hasBreaking: true, wantMajor: 2, wantMinor: 0, wantPatch: 0},
		{name: "feat overrides fix", hasFeat: true, hasFix: true, wantMajor: 1, wantMinor: 3, wantPatch: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := calc.NextVersion(base, tt.hasFeat, tt.hasFix, tt.hasBreaking)
			if tt.wantNil {
				if next != nil {
					t.Errorf("expected nil, got %v", next)
				}
				return
			}
			if next == nil {
				t.Fatal("expected non-nil version")
			}
			if next.Major != tt.wantMajor || next.Minor != tt.wantMinor || next.Patch != tt.wantPatch {
				t.Errorf("got %d.%d.%d want %d.%d.%d",
					next.Major, next.Minor, next.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestNextVersion_ZeroBase(t *testing.T) {
	calc := NewCalculator()
	base := &Version{}
	next := calc.NextVersion(base, true, false, false)
	if next == nil {
		t.Fatal("expected non-nil version")
	}
	if next.Major != 0 || next.Minor != 1 || next.Patch != 0 {
		t.Errorf("got %d.%d.%d want 0.1.0", next.Major, next.Minor, next.Patch)
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		v    Version
		want string
	}{
		{Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{Version{Major: 0, Minor: 0, Patch: 0}, "0.0.0"},
		{Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta.1"}, "1.2.3-beta.1"},
		{Version{Major: 1, Minor: 2, Patch: 3, Metadata: "build.1"}, "1.2.3+build.1"},
		{Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha", Metadata: "001"}, "1.2.3-alpha+001"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBumpFromRules(t *testing.T) {
	rules := []RuleEntry{
		{Type: "feat", Bump: "minor"},
		{Type: "fix", Bump: "patch"},
		{Type: "perf", Bump: "patch"},
		{Type: "docs", Bump: "patch"},
	}

	tests := []struct {
		name        string
		commits     []CommitInfo
		hasBreaking bool
		want        string
	}{
		{"no commits", nil, false, ""},
		{"only feat", []CommitInfo{{Type: "feat"}}, false, "minor"},
		{"only fix", []CommitInfo{{Type: "fix"}}, false, "patch"},
		{"feat+fix = minor", []CommitInfo{{Type: "feat"}, {Type: "fix"}}, false, "minor"},
		{"breaking overrides all", []CommitInfo{{Type: "feat"}}, true, "major"},
		{"breaking with no commits", nil, true, "major"},
		{"unknown type", []CommitInfo{{Type: "chore"}}, false, ""},
		{"docs patch", []CommitInfo{{Type: "docs"}}, false, "patch"},
		{"scope ignored when rule has no scope", []CommitInfo{{Type: "feat", Scope: "api"}}, false, "minor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BumpFromRules(tt.commits, rules, tt.hasBreaking)
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBumpFromRules_WithScope(t *testing.T) {
	rules := []RuleEntry{
		{Type: "deps", Scope: "major", Bump: "major"},
		{Type: "deps", Scope: "minor", Bump: "minor"},
		{Type: "deps", Scope: "patch", Bump: "patch"},
		{Type: "feat", Bump: "minor"},
	}

	tests := []struct {
		name        string
		commits     []CommitInfo
		hasBreaking bool
		want        string
	}{
		{
			name:    "deps(major) triggers major",
			commits: []CommitInfo{{Type: "deps", Scope: "major"}},
			want:    "major",
		},
		{
			name:    "deps(minor) triggers minor",
			commits: []CommitInfo{{Type: "deps", Scope: "minor"}},
			want:    "minor",
		},
		{
			name:    "deps(patch) triggers patch",
			commits: []CommitInfo{{Type: "deps", Scope: "patch"}},
			want:    "patch",
		},
		{
			name:    "deps with unknown scope matches no rule",
			commits: []CommitInfo{{Type: "deps", Scope: "security"}},
			want:    "",
		},
		{
			name:    "scope rule wins over type-only rule when scope matches",
			commits: []CommitInfo{{Type: "deps", Scope: "major"}, {Type: "feat"}},
			want:    "major",
		},
		{
			name:    "highest bump wins across scope and non-scope rules",
			commits: []CommitInfo{{Type: "deps", Scope: "patch"}, {Type: "feat"}},
			want:    "minor",
		},
		{
			name:        "breaking still overrides scope rules",
			commits:     []CommitInfo{{Type: "deps", Scope: "patch"}},
			hasBreaking: true,
			want:        "major",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BumpFromRules(tt.commits, rules, tt.hasBreaking)
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBumpFromRules_ScopeNone(t *testing.T) {
	rules := []RuleEntry{
		{Type: "chore", ScopeNone: true, Bump: "patch"}, // chore with NO scope → patch
		{Type: "feat", Bump: "minor"},                   // feat regardless of scope → minor
	}

	tests := []struct {
		name    string
		commits []CommitInfo
		want    string
	}{
		{
			name:    "chore without scope matches ScopeNone rule",
			commits: []CommitInfo{{Type: "chore", Scope: ""}},
			want:    "patch",
		},
		{
			name:    "chore with scope does NOT match ScopeNone rule",
			commits: []CommitInfo{{Type: "chore", Scope: "deps"}},
			want:    "",
		},
		{
			name:    "feat with scope matches type-only rule",
			commits: []CommitInfo{{Type: "feat", Scope: "api"}},
			want:    "minor",
		},
		{
			name:    "feat without scope matches type-only rule",
			commits: []CommitInfo{{Type: "feat", Scope: ""}},
			want:    "minor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BumpFromRules(tt.commits, rules, false)
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestNextVersionForBranch_MaintenancePatchOnly(t *testing.T) {
	c := NewCalculator()
	cur := &Version{Major: 1, Minor: 2, Patch: 3}

	// Breaking on maintenance branch → still only patch bump
	got := c.NextVersionForBranch(cur, false, false, true, true)
	if got == nil || got.String() != "1.2.4" {
		t.Errorf("expected 1.2.4, got %v", got)
	}

	// feat on maintenance branch → patch only
	got = c.NextVersionForBranch(cur, true, false, false, true)
	if got == nil || got.String() != "1.2.4" {
		t.Errorf("expected 1.2.4 for feat on maintenance, got %v", got)
	}

	// fix on maintenance branch → patch
	got = c.NextVersionForBranch(cur, false, true, false, true)
	if got == nil || got.String() != "1.2.4" {
		t.Errorf("expected 1.2.4 for fix on maintenance, got %v", got)
	}

	// no releasable commits on maintenance → nil
	got = c.NextVersionForBranch(cur, false, false, false, true)
	if got != nil {
		t.Errorf("expected nil for no commits on maintenance, got %v", got)
	}
}

func TestNextVersionForBranch_NonMaintenanceDelegates(t *testing.T) {
	c := NewCalculator()
	cur := &Version{Major: 1, Minor: 2, Patch: 3}

	// feat on normal branch → minor bump
	got := c.NextVersionForBranch(cur, true, false, false, false)
	if got == nil || got.String() != "1.3.0" {
		t.Errorf("expected 1.3.0 on normal branch, got %v", got)
	}

	// breaking on normal branch → major bump
	got = c.NextVersionForBranch(cur, false, false, true, false)
	if got == nil || got.String() != "2.0.0" {
		t.Errorf("expected 2.0.0 on normal branch, got %v", got)
	}
}

func TestNextPrereleaseVersion_FirstRelease(t *testing.T) {
	c := NewCalculator()
	cur := &Version{Major: 1, Minor: 2, Patch: 3}

	// feat → next stable would be 1.3.0, first beta = 1.3.0-beta.1
	got := c.NextPrereleaseVersion(cur, true, false, false, "beta")
	if got == nil || got.String() != "1.3.0-beta.1" {
		t.Errorf("expected 1.3.0-beta.1, got %v", got)
	}

	// fix → next stable would be 1.2.4, first rc = 1.2.4-rc.1
	got = c.NextPrereleaseVersion(cur, false, true, false, "rc")
	if got == nil || got.String() != "1.2.4-rc.1" {
		t.Errorf("expected 1.2.4-rc.1, got %v", got)
	}
}

func TestNextPrereleaseVersion_Increment(t *testing.T) {
	c := NewCalculator()
	// Already at 1.3.0-beta.2 — same base, increment counter
	cur := &Version{Major: 1, Minor: 3, Patch: 0, Prerelease: "beta.2"}

	got := c.NextPrereleaseVersion(cur, true, false, false, "beta")
	if got == nil || got.String() != "1.3.0-beta.3" {
		t.Errorf("expected 1.3.0-beta.3, got %v", got)
	}
}

func TestNextPrereleaseVersion_DifferentChannel(t *testing.T) {
	c := NewCalculator()
	// Currently on alpha, switching to beta for same base
	cur := &Version{Major: 1, Minor: 3, Patch: 0, Prerelease: "alpha.5"}

	got := c.NextPrereleaseVersion(cur, true, false, false, "beta")
	// Different channel, so start fresh beta.1
	if got == nil || got.String() != "1.3.0-beta.1" {
		t.Errorf("expected 1.3.0-beta.1 for new channel, got %v", got)
	}
}

func TestNextPrereleaseVersion_Breaking(t *testing.T) {
	c := NewCalculator()
	cur := &Version{Major: 1, Minor: 2, Patch: 3}

	got := c.NextPrereleaseVersion(cur, false, false, true, "alpha")
	if got == nil || got.String() != "2.0.0-alpha.1" {
		t.Errorf("expected 2.0.0-alpha.1, got %v", got)
	}
}

func TestNextPrereleaseVersion_NoChannel(t *testing.T) {
	c := NewCalculator()
	cur := &Version{Major: 1, Minor: 2, Patch: 3}

	// Empty channel = stable release
	got := c.NextPrereleaseVersion(cur, true, false, false, "")
	if got == nil || got.String() != "1.3.0" {
		t.Errorf("expected 1.3.0 for empty channel, got %v", got)
	}
}

func TestNextPrereleaseVersion_BreakingDuringBeta(t *testing.T) {
	c := NewCalculator()
	// Already at 1.3.0-beta.2, breaking change comes in → 2.0.0-beta.1
	cur := &Version{Major: 1, Minor: 3, Patch: 0, Prerelease: "beta.2"}

	got := c.NextPrereleaseVersion(cur, false, false, true, "beta")
	if got == nil || got.String() != "2.0.0-beta.1" {
		t.Errorf("expected 2.0.0-beta.1 for breaking during beta, got %v", got)
	}
}

func TestForcePatch(t *testing.T) {
	c := NewCalculator()

	tests := []struct {
		name    string
		current *Version
		want    string
	}{
		{"from stable", &Version{Major: 1, Minor: 2, Patch: 3}, "1.2.4"},
		{"from zero", &Version{}, "0.0.1"},
		{"after minor", &Version{Major: 2, Minor: 0, Patch: 0}, "2.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.ForcePatch(tt.current)
			if got == nil || got.String() != tt.want {
				t.Errorf("got %v, want %s", got, tt.want)
			}
		})
	}
}

func TestApplyCeiling_PatchBelowCeiling(t *testing.T) {
	current := &Version{Major: 1, Minor: 2, Patch: 3}
	next := &Version{Major: 1, Minor: 2, Patch: 4}
	ceiling := &Version{Major: 1, Minor: 3, Patch: 0}

	got, skipped, err := ApplyCeiling(current, next, ceiling, "clamp")
	if err != nil {
		t.Fatalf("ApplyCeiling() error = %v", err)
	}
	if skipped {
		t.Fatal("expected skipped=false")
	}
	if got == nil || got.String() != "1.2.4" {
		t.Fatalf("got %v, want 1.2.4", got)
	}
}

func TestApplyCeiling_MinorClampedToPatch(t *testing.T) {
	current := &Version{Major: 1, Minor: 2, Patch: 3}
	next := &Version{Major: 1, Minor: 3, Patch: 0}
	ceiling := &Version{Major: 1, Minor: 3, Patch: 0}

	got, skipped, err := ApplyCeiling(current, next, ceiling, "clamp")
	if err != nil {
		t.Fatalf("ApplyCeiling() error = %v", err)
	}
	if skipped {
		t.Fatal("expected skipped=false")
	}
	if got == nil || got.String() != "1.2.4" {
		t.Fatalf("got %v, want 1.2.4", got)
	}
}

func TestApplyCeiling_MajorClampedToMinorThenPatch(t *testing.T) {
	current := &Version{Major: 1, Minor: 9, Patch: 9}
	next := &Version{Major: 2, Minor: 0, Patch: 0}
	ceiling := &Version{Major: 1, Minor: 10, Patch: 0}

	got, skipped, err := ApplyCeiling(current, next, ceiling, "clamp")
	if err != nil {
		t.Fatalf("ApplyCeiling() error = %v", err)
	}
	if skipped {
		t.Fatal("expected skipped=false")
	}
	if got == nil || got.String() != "1.9.10" {
		t.Fatalf("got %v, want 1.9.10", got)
	}
}

func TestApplyCeiling_SkipStrategy(t *testing.T) {
	current := &Version{Major: 1, Minor: 2, Patch: 3}
	next := &Version{Major: 1, Minor: 3, Patch: 0}
	ceiling := &Version{Major: 1, Minor: 3, Patch: 0}

	got, skipped, err := ApplyCeiling(current, next, ceiling, "skip")
	if err != nil {
		t.Fatalf("ApplyCeiling() error = %v", err)
	}
	if !skipped {
		t.Fatal("expected skipped=true")
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestApplyCeiling_ErrorStrategy(t *testing.T) {
	current := &Version{Major: 1, Minor: 2, Patch: 3}
	next := &Version{Major: 1, Minor: 3, Patch: 0}
	ceiling := &Version{Major: 1, Minor: 3, Patch: 0}

	got, skipped, err := ApplyCeiling(current, next, ceiling, "error")
	if err == nil || !strings.Contains(err.Error(), "version ceiling reached") {
		t.Fatalf("expected ceiling error, got %v", err)
	}
	if skipped {
		t.Fatal("expected skipped=false")
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestApplyCeiling_AlreadyAtCeilingSkips(t *testing.T) {
	current := &Version{Major: 1, Minor: 2, Patch: 3}
	next := &Version{Major: 1, Minor: 2, Patch: 4}
	ceiling := &Version{Major: 1, Minor: 2, Patch: 4}

	got, skipped, err := ApplyCeiling(current, next, ceiling, "clamp")
	if err != nil {
		t.Fatalf("ApplyCeiling() error = %v", err)
	}
	if !skipped {
		t.Fatal("expected skipped=true")
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
