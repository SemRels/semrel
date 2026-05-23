// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package config provides .semrel.yaml configuration parsing.
// See: https://github.com/GoSemantics/semrel/issues/4
package config

// Config represents the semrel configuration.
type Config struct {
	Version  string         `yaml:"version"`
	Branches []BranchConfig `yaml:"branches"`
	Release  ReleaseConfig  `yaml:"release"`
	Plugins  []PluginConfig `yaml:"plugins,omitempty"`
}

// BranchConfig configures release behavior per branch.
type BranchConfig struct {
	Name       string `yaml:"name"`
	Prerelease bool   `yaml:"prerelease"`
}

// ReleaseConfig configures release rules.
type ReleaseConfig struct {
	Rules []ReleaseRule `yaml:"rules"`
}

// ReleaseRule maps commit type to version bump.
type ReleaseRule struct {
	Type string `yaml:"type"`
	Bump string `yaml:"bump"` // major, minor, patch
}

// PluginConfig configures a plugin.
type PluginConfig struct {
	Name string                 `yaml:"name"`
	Path string                 `yaml:"path,omitempty"`
	Args map[string]interface{} `yaml:"args,omitempty"`
}

// LoadConfig loads configuration from .semrel.yaml.
// Issue: https://github.com/GoSemantics/semrel/issues/4
func LoadConfig(path string) (*Config, error) {
	// TODO: Implement YAML parsing
	panic("not implemented")
}
