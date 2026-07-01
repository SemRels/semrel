// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
	return path
}

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

func TestLoadConfig_TOML(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.toml", `
tag_prefix = "v"

[[branches]]
name = "main"

[[branches]]
name = "next"
prerelease = "next"

[[rules]]
type = "feat"
bump = "minor"

[[rules]]
type = "fix"
bump = "patch"

[[plugins]]
uses = "git"
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if len(cfg.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(cfg.Branches))
	}
	if cfg.TagPrefix != "v" {
		t.Fatalf("expected tagPrefix=v, got %q", cfg.TagPrefix)
	}
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].Uses != "git" {
		t.Fatalf("unexpected plugins: %+v", cfg.Plugins)
	}
}

func TestLoadConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.json", `{
		"branches": [{"name": "main"}],
		"tag_prefix": "v",
		"rules": [{"type": "feat", "bump": "minor"}],
		"plugins": [{"uses": "git", "args": {"enabled": true}}]
	}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if len(cfg.Branches) != 1 || cfg.Branches[0].Name != "main" {
		t.Fatalf("unexpected branches: %+v", cfg.Branches)
	}
	if got := cfg.Plugins[0].Args["enabled"]; got != true {
		t.Fatalf("expected enabled=true, got %#v", got)
	}
}

func TestFindConfigFile_FindsYAMLFirst(t *testing.T) {
	dir := t.TempDir()
	want := writeConfigFile(t, dir, ".semrel.yaml", "branches:\n  - name: main\n")
	writeConfigFile(t, dir, ".semrel.toml", "[[branches]]\nname = \"main\"\n")

	got, err := FindConfigFile(dir)
	if err != nil {
		t.Fatalf("FindConfigFile(): %v", err)
	}
	if got != want {
		t.Fatalf("FindConfigFile() = %q, want %q", got, want)
	}
}

func TestFindConfigFile_FindsTOMLWhenNoYAMLExists(t *testing.T) {
	dir := t.TempDir()
	want := writeConfigFile(t, dir, ".semrel.toml", "[[branches]]\nname = \"main\"\n")

	got, err := FindConfigFile(dir)
	if err != nil {
		t.Fatalf("FindConfigFile(): %v", err)
	}
	if got != want {
		t.Fatalf("FindConfigFile() = %q, want %q", got, want)
	}
}

func TestFindConfigFile_ErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()

	_, err := FindConfigFile(dir)
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
}

func TestLoadConfig_UnknownExtensionFallsBackToYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.conf", "branches:\n  - name: main\n")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if len(cfg.Branches) != 1 || cfg.Branches[0].Name != "main" {
		t.Fatalf("unexpected branches: %+v", cfg.Branches)
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

func TestValidate_DuplicateRuleTypeAndScope(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "deps", Scope: "major", Bump: "major"},
			{Type: "deps", Scope: "major", Bump: "minor"}, // duplicate (type+scope)
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for duplicate rule type+scope combination")
	}
}

func TestValidate_SameTypeWithDifferentScopesIsValid(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "deps", Scope: "major", Bump: "major"},
			{Type: "deps", Scope: "minor", Bump: "minor"},
			{Type: "deps", Scope: "patch", Bump: "patch"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config for same type with different scopes, got: %v", err)
	}
}

func TestValidate_ScopeFalseIsValid(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "chore", Scope: false, Bump: "patch"},
			{Type: "feat", Bump: "minor"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected scope:false to be valid, got: %v", err)
	}
}

func TestValidate_ScopeFalseDuplicateIsError(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "chore", Scope: false, Bump: "patch"},
			{Type: "chore", Scope: false, Bump: "minor"}, // duplicate
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for duplicate scope:false rule")
	}
}

func TestValidate_ScopeFalseAndScopeStringAreDistinct(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "chore", Scope: false, Bump: "patch"},  // scopeless commits
			{Type: "chore", Scope: "deps", Bump: "minor"}, // scoped commits
			{Type: "chore", Bump: "patch"},                // any commit (inc. scoped)
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected scope:false and scope:string to be distinct keys, got: %v", err)
	}
}

func TestValidate_ScopeInvalidTypeIsError(t *testing.T) {
	cfg := &Config{
		TagPrefix: "v",
		Rules: []ReleaseRule{
			{Type: "feat", Scope: true, Bump: "minor"}, // scope:true is invalid
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for scope:true")
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

func TestValidate_VersionCeilingAccepted(t *testing.T) {
	cfg := &Config{VersionCeiling: "1.2.3"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid version_ceiling, got %v", err)
	}
}

func TestValidate_InvalidVersionCeilingRejected(t *testing.T) {
	cfg := &Config{VersionCeiling: "not-a-version"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "version_ceiling") {
		t.Fatalf("expected version_ceiling validation error, got %v", err)
	}
}

func TestValidate_InvalidCeilingStrategyRejected(t *testing.T) {
	cfg := &Config{CeilingStrategy: "explode"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "ceiling_strategy") {
		t.Fatalf("expected ceiling_strategy validation error, got %v", err)
	}
}

func TestValidate_ValidCeilingStrategiesAccepted(t *testing.T) {
	for _, strategy := range []string{"clamp", "skip", "error"} {
		t.Run(strategy, func(t *testing.T) {
			cfg := &Config{CeilingStrategy: strategy}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("expected valid ceiling_strategy %q, got %v", strategy, err)
			}
		})
	}
}
