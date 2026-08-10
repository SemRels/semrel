// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package config provides semrel configuration parsing.
// See: https://github.com/SemRels/semrel/issues/4
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	"github.com/SemRels/semrel/pkg/semver"
)

// CurrentSchemaVersion is the schema version that this build of semrel understands.
const CurrentSchemaVersion = 1

// Config represents the semrel configuration.
type Config struct {
	// SchemaVersion is the semrel config schema version.
	// When absent (0), semrel treats the config as schema version 1.
	// Use `semrel migrate` to stamp and upgrade the config to the current schema.
	SchemaVersion int `yaml:"schemaVersion,omitempty" toml:"schema_version" json:"schema_version,omitempty"`

	Branches        []BranchConfig `yaml:"branches" toml:"branches" json:"branches"`
	TagPrefix       string         `yaml:"tagPrefix" toml:"tag_prefix" json:"tag_prefix"`
	Rules           []ReleaseRule  `yaml:"rules" toml:"rules" json:"rules"`
	Plugins         []PluginConfig `yaml:"plugins,omitempty" toml:"plugins" json:"plugins,omitempty"`
	VersionCeiling  string         `yaml:"version_ceiling,omitempty" toml:"version_ceiling" json:"version_ceiling,omitempty"`
	CeilingStrategy string         `yaml:"ceiling_strategy,omitempty" toml:"ceiling_strategy" json:"ceiling_strategy,omitempty"`
	// CommitChangelog controls whether semrel commits CHANGELOG.md back to the repo
	// before creating the release tag. Default: true.
	CommitChangelog *bool `yaml:"commit_changelog,omitempty" toml:"commit_changelog" json:"commit_changelog,omitempty"`
	// TagExistsStrategy controls what semrel does when the computed tag already exists locally.
	// "update-changelog" (default): updates CHANGELOG.md, commits, and exits without error.
	// "skip": exits silently without any changes.
	// "error": returns a non-zero exit code.
	TagExistsStrategy string `yaml:"tag_exists_strategy,omitempty" toml:"tag_exists_strategy" json:"tag_exists_strategy,omitempty"`
	// Workspace configures multi-package (monorepo) workspace mode.
	// When set, `semrel workspace release` orchestrates all configured packages.
	Workspace *WorkspaceConfig `yaml:"workspace,omitempty" toml:"workspace" json:"workspace,omitempty"`
}

// WorkspaceConfig enables multi-package workspace (monorepo) mode.
// Packages are released via `semrel workspace release`.
type WorkspaceConfig struct {
	// Strategy is the versioning strategy:
	//   "independent" (default) — each package gets its own semver version.
	//   "lockstep" — all packages share the same version and are tagged together.
	Strategy string `yaml:"strategy,omitempty" toml:"strategy" json:"strategy,omitempty"`
	// Packages is an explicit list of workspace packages.
	// Mutually exclusive with Pattern; Packages takes precedence when both are set.
	Packages []WorkspacePackageRef `yaml:"packages,omitempty" toml:"packages" json:"packages,omitempty"`
	// Pattern is a glob pattern relative to the repository root that discovers
	// package directories automatically (e.g. "packages/*" or "services/**").
	// Each matched directory is treated as an independent package.
	Pattern string `yaml:"pattern,omitempty" toml:"pattern" json:"pattern,omitempty"`
	// FailFast stops the workspace release on the first package failure.
	// Default false: all packages are attempted and errors are collected.
	FailFast bool `yaml:"fail_fast,omitempty" toml:"fail_fast" json:"fail_fast,omitempty"`
}

// WorkspacePackageRef references a single package inside a workspace.
type WorkspacePackageRef struct {
	// Path is the directory of the package relative to the repository root.
	Path string `yaml:"path" toml:"path" json:"path"`
	// TagPrefix overrides the package-level git tag prefix.
	// Defaults to "<path>@v" for independent mode (e.g. "packages/api@v").
	TagPrefix string `yaml:"tagPrefix,omitempty" toml:"tag_prefix" json:"tag_prefix,omitempty"`
	// DependsOn lists other package paths this package depends on.
	// Dependent packages are released only after all their dependencies succeed.
	DependsOn []string `yaml:"dependsOn,omitempty" toml:"depends_on" json:"depends_on,omitempty"`
}

// BranchConfig configures release behavior per branch.
// For maintenance branches, set Maintenance: true or use a pattern like "1.x" / "1.2.x".
// See: https://github.com/SemRels/semrel/issues/95
type BranchConfig struct {
	Name       string `yaml:"name" toml:"name" json:"name"`
	Prerelease string `yaml:"prerelease,omitempty" toml:"prerelease" json:"prerelease,omitempty"`
	// Maintenance marks this branch as a maintenance branch.
	// On maintenance branches only patch bumps are allowed.
	// Can also be inferred automatically from the branch name pattern (N.x, N.M.x).
	Maintenance bool `yaml:"maintenance,omitempty" toml:"maintenance" json:"maintenance,omitempty"`
}

// maintenancePattern matches branch names like "1.x", "1.2.x", "2.x".
var maintenancePattern = regexp.MustCompile(`^\d+(\.\d+)?\.x$`)

// IsMaintenance reports whether the given branch name is a maintenance branch.
// It checks the Maintenance flag first, then auto-detects the N.x / N.M.x pattern.
func IsMaintenance(branchName string, cfg BranchConfig) bool {
	if cfg.Maintenance {
		return true
	}
	return maintenancePattern.MatchString(branchName)
}

// MatchesBranchPattern reports whether branchName matches the given pattern.
// Supports exact matches and simple glob-style wildcards (*).
func MatchesBranchPattern(branchName, pattern string) bool {
	if pattern == branchName {
		return true
	}
	// Convert glob to regexp: escape dots, replace * with .*
	regexStr := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
	re, err := regexp.Compile(regexStr)
	if err != nil {
		return false
	}
	return re.MatchString(branchName)
}

// FindBranchConfig returns the BranchConfig that matches the given branch name,
// or nil if no branch in the config matches.
func (c *Config) FindBranchConfig(branchName string) *BranchConfig {
	for i := range c.Branches {
		if MatchesBranchPattern(branchName, c.Branches[i].Name) {
			return &c.Branches[i]
		}
	}
	return nil
}

// ReleaseRule maps a commit type (and optionally scope) to a version bump level.
//
// Scope matching behaviour:
//   - Scope omitted (nil): matches commits of the given type regardless of scope.
//   - Scope is a string:   matches only commits with that exact type AND scope.
//   - Scope is false:      matches only commits of the given type that carry NO scope.
type ReleaseRule struct {
	Type   string      `yaml:"type" toml:"type" json:"type"`
	Scope  interface{} `yaml:"scope,omitempty" toml:"scope,omitempty" json:"scope,omitempty"`
	Bump   string      `yaml:"bump" toml:"bump" json:"bump"` // major, minor, patch
	Hidden bool        `yaml:"hidden,omitempty" toml:"hidden" json:"hidden,omitempty"`
}

// PluginConfig configures a plugin.
type PluginConfig struct {
	Uses string                 `yaml:"uses" toml:"uses" json:"uses"`
	Name string                 `yaml:"name,omitempty" toml:"name" json:"name,omitempty"`
	Path string                 `yaml:"path,omitempty" toml:"path" json:"path,omitempty"`
	Args map[string]interface{} `yaml:"args,omitempty" toml:"args" json:"args,omitempty"`
	// Phase controls when the plugin runs:
	// "condition" — runs before any commit is analyzed or tag created (gate check).
	// "release"   — runs after the tag is created (default; provider, hook, updater plugins).
	Phase string `yaml:"phase,omitempty" toml:"phase" json:"phase,omitempty"`
}

// Validate checks the configuration for semantic errors beyond YAML parsing.
// Returns a multi-error string describing all validation failures, or nil if valid.
// See: https://github.com/SemRels/semrel/issues/15
func (c *Config) Validate() error {
	var errs []string

	// Tag prefix must not contain spaces or look like a semver component
	if strings.ContainsAny(c.TagPrefix, " \t\n") {
		errs = append(errs, "tagPrefix must not contain whitespace")
	}

	// Branch names must not be empty
	seenBranches := make(map[string]bool)
	for i, b := range c.Branches {
		if strings.TrimSpace(b.Name) == "" {
			errs = append(errs, fmt.Sprintf("branches[%d]: name must not be empty", i))
			continue
		}
		if seenBranches[b.Name] {
			errs = append(errs, fmt.Sprintf("branches[%d]: duplicate branch name %q", i, b.Name))
		}
		seenBranches[b.Name] = true
	}

	// Release rules: type and bump must be valid, (type, scope) pairs must be unique
	validBumps := map[string]bool{"major": true, "minor": true, "patch": true}
	seenRules := make(map[string]bool)
	for i, r := range c.Rules {
		if strings.TrimSpace(r.Type) == "" {
			errs = append(errs, fmt.Sprintf("rules[%d]: type must not be empty", i))
		}
		if !validBumps[r.Bump] {
			errs = append(errs, fmt.Sprintf("rules[%d]: bump %q is not valid (must be major, minor, or patch)", i, r.Bump))
		}
		// Validate and canonicalise the scope value for duplicate detection.
		scopeKey, scopeErr := canonicaliseScopeKey(r.Scope)
		if scopeErr != nil {
			errs = append(errs, fmt.Sprintf("rules[%d]: scope %v", i, scopeErr))
		}
		key := r.Type + "\x00" + scopeKey
		if seenRules[key] {
			switch v := r.Scope.(type) {
			case string:
				errs = append(errs, fmt.Sprintf("rules[%d]: duplicate rule for type %q with scope %q", i, r.Type, v))
			case bool:
				errs = append(errs, fmt.Sprintf("rules[%d]: duplicate rule for type %q with scope false", i, r.Type))
			default:
				errs = append(errs, fmt.Sprintf("rules[%d]: duplicate rule type %q", i, r.Type))
			}
		}
		seenRules[key] = true
	}

	// Plugins: uses or path must be set
	for i, p := range c.Plugins {
		if strings.TrimSpace(p.Uses) == "" && strings.TrimSpace(p.Path) == "" {
			errs = append(errs, fmt.Sprintf("plugins[%d]: either 'uses' or 'path' must be set", i))
		}
		if p.Phase != "" {
			validPhases := map[string]bool{"condition": true, "pre-tag": true, "release": true, "generator": true}
			if !validPhases[p.Phase] {
				errs = append(errs, fmt.Sprintf("plugins[%d]: phase %q is not valid (must be condition, generator, pre-tag, or release)", i, p.Phase))
			}
		}
	}

	if c.VersionCeiling != "" {
		if _, err := semver.ParseVersion(c.VersionCeiling); err != nil {
			errs = append(errs, fmt.Sprintf("version_ceiling must be a valid semver: %v", err))
		}
	}

	if c.CeilingStrategy != "" {
		validStrategies := map[string]bool{"clamp": true, "skip": true, "error": true}
		if !validStrategies[c.CeilingStrategy] {
			errs = append(errs, fmt.Sprintf("ceiling_strategy %q is not valid (must be clamp, skip, or error)", c.CeilingStrategy))
		}
	}

	if c.TagExistsStrategy != "" {
		validTagStrategies := map[string]bool{"update-changelog": true, "skip": true, "error": true}
		if !validTagStrategies[c.TagExistsStrategy] {
			errs = append(errs, fmt.Sprintf("tag_exists_strategy %q is not valid (must be update-changelog, skip, or error)", c.TagExistsStrategy))
		}
	}

	if ws := c.Workspace; ws != nil {
		if ws.Strategy != "" {
			validStrategies := map[string]bool{"independent": true, "lockstep": true}
			if !validStrategies[ws.Strategy] {
				errs = append(errs, fmt.Sprintf("workspace.strategy %q is not valid (must be independent or lockstep)", ws.Strategy))
			}
		}
		if ws.Pattern != "" && len(ws.Packages) > 0 {
			errs = append(errs, "workspace: 'pattern' and 'packages' are mutually exclusive; use one or the other")
		}
		for i, pkg := range ws.Packages {
			if strings.TrimSpace(pkg.Path) == "" {
				errs = append(errs, fmt.Sprintf("workspace.packages[%d]: path must not be empty", i))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// LoadConfig loads configuration from the given file path.
// Issue: https://github.com/SemRels/semrel/issues/4
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var cfg Config
	switch ext {
	case ".toml":
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return nil, fmt.Errorf("parsing TOML config %s: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing JSON config %s: %w", path, err)
		}
	default:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing YAML config %s: %w", path, err)
		}
	}

	// Apply defaults
	if cfg.TagPrefix == "" {
		cfg.TagPrefix = "v"
	}
	if len(cfg.Rules) == 0 {
		cfg.Rules = defaultRules()
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// FindConfigFile returns the first semrel config file found in dir.
func FindConfigFile(dir string) (string, error) {
	names := []string{".semrel.yaml", ".semrel.yml", ".semrel.toml", ".semrel.json"}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no semrel config file found in %s (tried: %s)", dir, strings.Join(names, ", "))
}

func defaultRules() []ReleaseRule {
	return []ReleaseRule{
		{Type: "feat", Bump: "minor"},
		{Type: "fix", Bump: "patch"},
		{Type: "perf", Bump: "patch"},
		{Type: "revert", Bump: "patch"},
	}
}

// ShouldCommitChangelog returns true unless commit_changelog is explicitly set to false.
func (c *Config) ShouldCommitChangelog() bool {
	if c.CommitChangelog == nil {
		return true // default: commit
	}
	return *c.CommitChangelog
}

// ResolvedTagExistsStrategy returns the effective tag_exists_strategy (default: "update-changelog").
func (c *Config) ResolvedTagExistsStrategy() string {
	if c.TagExistsStrategy == "" {
		return "update-changelog"
	}
	return c.TagExistsStrategy
}

// canonicaliseScopeKey returns a stable string key for a scope value used in
// duplicate-detection and rule-matching. Valid values are:
//   - nil / absent  → ""           (match any scope)
//   - string        → "s:<value>"  (match exact scope)
//   - bool false    → "none"       (match scopeless commits only)
//
// Any other value is an error.
func canonicaliseScopeKey(scope interface{}) (string, error) {
	switch v := scope.(type) {
	case nil:
		return "", nil
	case string:
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("must not be an empty string; omit scope to match any commit, or use false to match scopeless commits")
		}
		return "s:" + v, nil
	case bool:
		if v {
			return "", fmt.Errorf("as a boolean must be false (matches commits with no scope)")
		}
		return "none", nil
	default:
		return "", fmt.Errorf("must be a string or false, got %T", scope)
	}
}
