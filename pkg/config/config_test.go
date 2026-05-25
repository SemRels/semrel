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

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/semrel.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
