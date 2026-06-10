// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/GoSemantics/semrel/pkg/config"
)

func TestConfigCommand_RegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Use == "config" {
			return
		}
	}
	t.Error("expected 'config' command registered in root")
}

func TestConfigCommand_HasSubcommands(t *testing.T) {
	root := NewRootCommand()
	var configCmd interface{ Commands() []*cobra.Command }
	for _, sub := range root.Commands() {
		if sub.Use == "config" {
			configCmd = sub
			break
		}
	}
	if configCmd == nil {
		t.Fatal("config command not found")
	}
	subs := map[string]bool{}
	for _, c := range configCmd.Commands() {
		subs[c.Use] = true
	}
	for _, expected := range []string{"init", "validate", "show", "set <key> <value>"} {
		if !subs[expected] {
			t.Errorf("expected subcommand %q, found: %v", expected, subs)
		}
	}
}

func TestRunConfigValidate_Valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	if err := runConfigValidate(cfgPath); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunConfigValidate_Invalid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: ""
`)
	if err := runConfigValidate(cfgPath); err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestRunConfigShow_YAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
tagPrefix: v
`)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(cfgPath, "text")

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "main") {
		t.Errorf("expected 'main' in output, got: %s", buf.String())
	}
}

func TestRunConfigShow_JSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigShow(cfgPath, "json")

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	var cfg config.Config
	if jsonErr := json.Unmarshal(buf.Bytes(), &cfg); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", jsonErr, buf.String())
	}
	if len(cfg.Branches) == 0 || cfg.Branches[0].Name != "main" {
		t.Errorf("expected branches[0].name=main, got: %+v", cfg.Branches)
	}
}

func TestRunConfigSet_TagPrefix(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
tagPrefix: v
`)
	if err := runConfigSet(cfgPath, "tagPrefix", "release/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TagPrefix != "release/" {
		t.Errorf("expected release/, got %q", cfg.TagPrefix)
	}
}

func TestRunConfigSet_BranchName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	if err := runConfigSet(cfgPath, "branches.0.name", "develop"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Branches[0].Name != "develop" {
		t.Errorf("expected develop, got %q", cfg.Branches[0].Name)
	}
}

func TestRunConfigSet_CommitChangelog(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	if err := runConfigSet(cfgPath, "commit_changelog", "false"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CommitChangelog == nil || *cfg.CommitChangelog != false {
		t.Errorf("expected commit_changelog=false, got %v", cfg.CommitChangelog)
	}
}

func TestRunConfigSet_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	err := runConfigSet(cfgPath, "does_not_exist", "value")
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestRunConfigSet_BranchesInvalidIndex(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	err := runConfigSet(cfgPath, "branches.notanindex.name", "develop")
	if err == nil {
		t.Error("expected error for non-integer index")
	}
}

func TestRunConfigInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".semrel.yaml")
	_ = os.WriteFile(cfgPath, []byte("branches:\n  - name: main\n"), 0o644)

	err := runConfigInit(cfgPath, true, false)
	if err == nil {
		t.Error("expected error when config already exists without --force")
	}
}

func TestRunConfigInit_Force(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".semrel.yaml")
	_ = os.WriteFile(cfgPath, []byte("branches:\n  - name: main\n"), 0o644)

	if err := runConfigInit(cfgPath, true, true); err != nil {
		t.Errorf("unexpected error with --force: %v", err)
	}
}

func TestRunConfigInit_NoInteractive(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".semrel.yaml")

	if err := runConfigInit(cfgPath, true, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Branches) == 0 {
		t.Error("expected at least one branch in default config")
	}
}

func TestMarshalConfigYAML_ContainsComment(t *testing.T) {
	cfg := defaultConfig()
	data, err := marshalConfigYAML(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "semrel.io/schema/v1.json") {
		t.Errorf("expected yaml-language-server schema directive, got: %s", out)
	}
	if !strings.Contains(out, "semrel configuration") {
		t.Errorf("expected header comment, got: %s", out)
	}
}

func TestApplyConfigKey_VersionCeiling(t *testing.T) {
	cfg := &config.Config{Branches: []config.BranchConfig{{Name: "main"}}}
	if err := applyConfigKey(cfg, "version_ceiling", "2.0.0"); err != nil {
		t.Fatal(err)
	}
	if cfg.VersionCeiling != "2.0.0" {
		t.Errorf("expected 2.0.0, got %q", cfg.VersionCeiling)
	}
}

func TestApplyConfigKey_BranchMaintenance(t *testing.T) {
	cfg := &config.Config{Branches: []config.BranchConfig{{Name: "1.x"}}}
	if err := applyConfigKey(cfg, "branches.0.maintenance", "true"); err != nil {
		t.Fatal(err)
	}
	if !cfg.Branches[0].Maintenance {
		t.Error("expected maintenance=true")
	}
}
