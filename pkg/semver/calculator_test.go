// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package semver

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input      string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantMeta   string
		wantErr    bool
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
	rules := map[string]string{
		"feat": "minor",
		"fix":  "patch",
		"perf": "patch",
		"docs": "patch",
	}

	tests := []struct {
		name        string
		types       []string
		hasBreaking bool
		want        string
	}{
		{"no commits", nil, false, ""},
		{"only feat", []string{"feat"}, false, "minor"},
		{"only fix", []string{"fix"}, false, "patch"},
		{"feat+fix = minor", []string{"feat", "fix"}, false, "minor"},
		{"breaking overrides all", []string{"feat"}, true, "major"},
		{"breaking with no commits", nil, true, "major"},
		{"unknown type", []string{"chore"}, false, ""},
		{"docs patch", []string{"docs"}, false, "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BumpFromRules(tt.types, rules, tt.hasBreaking)
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

