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
		wantErr    bool
	}{
		{input: "1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "v1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "0.0.0", wantMajor: 0, wantMinor: 0, wantPatch: 0},
		{input: "1.2.3-beta.1", wantMajor: 1, wantMinor: 2, wantPatch: 3, wantPre: "beta.1"},
		{input: "", wantMajor: 0, wantMinor: 0, wantPatch: 0},
		{input: "not-a-version", wantErr: true},
	}

	for _, tt := range tests {
		v, err := ParseVersion(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if v.Major != tt.wantMajor || v.Minor != tt.wantMinor || v.Patch != tt.wantPatch {
			t.Errorf("ParseVersion(%q): got %d.%d.%d want %d.%d.%d",
				tt.input, v.Major, v.Minor, v.Patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
		}
		if v.Prerelease != tt.wantPre {
			t.Errorf("ParseVersion(%q): Prerelease got %q want %q", tt.input, v.Prerelease, tt.wantPre)
		}
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

func TestVersionString(t *testing.T) {
	v := &Version{Major: 1, Minor: 2, Patch: 3}
	if s := v.String(); s != "1.2.3" {
		t.Errorf("got %q want %q", s, "1.2.3")
	}
	v.Prerelease = "beta.1"
	if s := v.String(); s != "1.2.3-beta.1" {
		t.Errorf("got %q want %q", s, "1.2.3-beta.1")
	}
}
