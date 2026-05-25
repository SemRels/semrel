// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
branches:
  - name: main
  - name: next
    prerelease: next
tagPrefix: "v"
rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
plugins:
  - uses: git
  - uses: changelog
`
	f, err := os.CreateTemp("", "semrel-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Branches) != 2 {
		t.Errorf("expected 2 branches, got %d", len(cfg.Branches))
	}
	if cfg.TagPrefix != "v" {
		t.Errorf("expected tagPrefix=v, got %q", cfg.TagPrefix)
	}
	if len(cfg.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Type != "feat" || cfg.Rules[0].Bump != "minor" {
		t.Errorf("unexpected first rule: %+v", cfg.Rules[0])
	}
	if len(cfg.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(cfg.Plugins))
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	yaml := "branches:\n  - name: main\n"
	f, err := os.CreateTemp("", "semrel-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.TagPrefix != "v" {
		t.Errorf("expected default tagPrefix=v, got %q", cfg.TagPrefix)
	}
	if len(cfg.Rules) == 0 {
		t.Error("expected default rules to be applied")
	}
}

func TestIsMaintenance_PatternDetection(t *testing.T) {
	tests := []struct {
		branch string
		want   bool
	}{
		{"1.x", true},
		{"1.2.x", true},
		{"12.3.x", true},
		{"main", false},
		{"next", false},
		{"feature/foo", false},
		{"1.x.y", false},
		{"1.2", false},
	}
	for _, tt := range tests {
		got := IsMaintenance(tt.branch, BranchConfig{Name: tt.branch})
		if got != tt.want {
			t.Errorf("IsMaintenance(%q) = %v, want %v", tt.branch, got, tt.want)
		}
	}
}

func TestIsMaintenance_ExplicitFlag(t *testing.T) {
	cfg := BranchConfig{Name: "support/v1", Maintenance: true}
	if !IsMaintenance("support/v1", cfg) {
		t.Error("expected Maintenance=true to be respected")
	}
	cfg2 := BranchConfig{Name: "main", Maintenance: false}
	if IsMaintenance("main", cfg2) {
		t.Error("expected Maintenance=false for main")
	}
}

func TestMatchesBranchPattern_Exact(t *testing.T) {
	if !MatchesBranchPattern("main", "main") {
		t.Error("exact match should work")
	}
	if MatchesBranchPattern("main", "next") {
		t.Error("non-match should return false")
	}
}

func TestMatchesBranchPattern_Wildcard(t *testing.T) {
	if !MatchesBranchPattern("release/1.0", "release/*") {
		t.Error("wildcard should match")
	}
	if !MatchesBranchPattern("feat/foo-bar", "feat/*") {
		t.Error("wildcard should match")
	}
	if MatchesBranchPattern("hotfix/1.0", "feat/*") {
		t.Error("wildcard should not match different prefix")
	}
}

func TestFindBranchConfig(t *testing.T) {
	cfg := &Config{
		Branches: []BranchConfig{
			{Name: "main"},
			{Name: "next", Prerelease: "next"},
			{Name: "1.x", Maintenance: true},
			{Name: "release/*"},
		},
	}

	if bc := cfg.FindBranchConfig("main"); bc == nil || bc.Name != "main" {
		t.Error("should find main branch")
	}
	if bc := cfg.FindBranchConfig("next"); bc == nil || bc.Prerelease != "next" {
		t.Error("should find next branch with prerelease")
	}
	if bc := cfg.FindBranchConfig("1.x"); bc == nil || !bc.Maintenance {
		t.Error("should find maintenance branch")
	}
	if bc := cfg.FindBranchConfig("release/2.0"); bc == nil {
		t.Error("should find release/* wildcard")
	}
	if bc := cfg.FindBranchConfig("unknown"); bc != nil {
		t.Error("should return nil for unknown branch")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Branches:  []BranchConfig{{Name: "main"}},
		Rules: []ReleaseRule{
			{Type: "feat", Bump: "minor"},
			{Type: "fix", Bump: "patch"},
		},
		Plugins: []PluginConfig{{Uses: "git"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidate_EmptyBranchName(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Branches:  []BranchConfig{{Name: ""}},
		Rules:     []ReleaseRule{{Type: "feat", Bump: "minor"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty branch name")
	}
}

func TestValidate_DuplicateBranch(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Branches:  []BranchConfig{{Name: "main"}, {Name: "main"}},
		Rules:     []ReleaseRule{{Type: "feat", Bump: "minor"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for duplicate branch name")
	}
}

func TestValidate_InvalidBump(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules:     []ReleaseRule{{Type: "feat", Bump: "invalid"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid bump value")
	}
}

func TestValidate_DuplicateRuleType(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "feat", Bump: "minor"},
			{Type: "feat", Bump: "major"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for duplicate rule type")
	}
}

func TestValidate_PluginMissingUsesAndPath(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules:     []ReleaseRule{{Type: "feat", Bump: "minor"}},
		Plugins:   []PluginConfig{{Args: map[string]interface{}{"key": "value"}}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for plugin missing uses and path")
	}
}

func TestValidate_TagPrefixWithSpace(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v ",
		Rules:     []ReleaseRule{{Type: "feat", Bump: "minor"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for tagPrefix with whitespace")
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v\t",
		Branches:  []BranchConfig{{Name: ""}},
		Rules: []ReleaseRule{
			{Type: "", Bump: "badvalue"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	// Should mention multiple issues
	errMsg := err.Error()
	if len(errMsg) < 50 {
		t.Errorf("expected detailed error message, got: %q", errMsg)
	}
}

func TestLoadConfig_MaintenanceBranch(t *testing.T) {
	yaml := `
branches:
  - name: main
  - name: 1.x
    maintenance: true
`
	f, err := os.CreateTemp("", "semrel-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	bc := cfg.FindBranchConfig("1.x")
	if bc == nil {
		t.Fatal("should find 1.x branch")
	}
	if !IsMaintenance("1.x", *bc) {
		t.Error("1.x should be a maintenance branch")
	}
}
