// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package config provides .semrel.yaml configuration parsing.
// See: https://github.com/SemRels/semrel/issues/4
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the semrel configuration.
type Config struct {
	Branches  []BranchConfig `yaml:"branches"`
	TagPrefix string         `yaml:"tagPrefix"`
	Rules     []ReleaseRule  `yaml:"rules"`
	Plugins   []PluginConfig `yaml:"plugins,omitempty"`
}

// BranchConfig configures release behavior per branch.
type BranchConfig struct {
	Name       string `yaml:"name"`
	Prerelease string `yaml:"prerelease,omitempty"`
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
