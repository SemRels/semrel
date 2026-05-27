// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package config provides .semrel.yaml configuration parsing.
// See: https://github.com/SemRels/semrel/issues/4
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/GoSemantics/semrel/pkg/semver"
	"gopkg.in/yaml.v3"
)

// Config represents the semrel configuration.
type Config struct {
	Branches        []BranchConfig `yaml:"branches"`
	TagPrefix       string         `yaml:"tagPrefix"`
	Rules           []ReleaseRule  `yaml:"rules"`
	Plugins         []PluginConfig `yaml:"plugins,omitempty"`
	VersionCeiling  string         `yaml:"version_ceiling,omitempty"`
	CeilingStrategy string         `yaml:"ceiling_strategy,omitempty"`
}

// BranchConfig configures release behavior per branch.
// For maintenance branches, set Maintenance: true or use a pattern like "1.x" / "1.2.x".
// See: https://github.com/SemRels/semrel/issues/95
type BranchConfig struct {
	Name       string `yaml:"name"`
	Prerelease string `yaml:"prerelease,omitempty"`
	// Maintenance marks this branch as a maintenance branch.
	// On maintenance branches only patch bumps are allowed.
	// Can also be inferred automatically from the branch name pattern (N.x, N.M.x).
	Maintenance bool `yaml:"maintenance,omitempty"`
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

// ReleaseRule maps commit type to version bump level.
type ReleaseRule struct {
	Type string `yaml:"type"`
	Bump string `yaml:"bump"` // major, minor, patch
}

// PluginConfig configures a plugin.
type PluginConfig struct {
	Uses string                 `yaml:"uses"`
	Path string                 `yaml:"path,omitempty"`
	Args map[string]interface{} `yaml:"args,omitempty"`
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

	// Release rules: type and bump must be valid
	validBumps := map[string]bool{"major": true, "minor": true, "patch": true}
	seenTypes := make(map[string]bool)
	for i, r := range c.Rules {
		if strings.TrimSpace(r.Type) == "" {
			errs = append(errs, fmt.Sprintf("rules[%d]: type must not be empty", i))
		}
		if !validBumps[r.Bump] {
			errs = append(errs, fmt.Sprintf("rules[%d]: bump %q is not valid (must be major, minor, or patch)", i, r.Bump))
		}
		if seenTypes[r.Type] {
			errs = append(errs, fmt.Sprintf("rules[%d]: duplicate rule type %q", i, r.Type))
		}
		seenTypes[r.Type] = true
	}

	// Plugins: uses or path must be set
	for i, p := range c.Plugins {
		if strings.TrimSpace(p.Uses) == "" && strings.TrimSpace(p.Path) == "" {
			errs = append(errs, fmt.Sprintf("plugins[%d]: either 'uses' or 'path' must be set", i))
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

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// LoadConfig loads configuration from the given YAML file path.
// Issue: https://github.com/SemRels/semrel/issues/4
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
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

func defaultRules() []ReleaseRule {
	return []ReleaseRule{
		{Type: "feat", Bump: "minor"},
		{Type: "fix", Bump: "patch"},
		{Type: "perf", Bump: "patch"},
		{Type: "revert", Bump: "patch"},
	}
}
