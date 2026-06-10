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

	"github.com/SemRels/semrel/pkg/config"
)

// writeTestConfig writes a minimal .semrel.yaml to dir and returns the file path.
func writeTestConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".semrel.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}

func TestCheckConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	_, cfg, check := checkConfig(cfgPath)
	if check.Status != "ok" {
		t.Errorf("expected ok, got %q: %s", check.Status, check.Message)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestCheckConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	// No config file in the temp dir.
	_, cfg, check := checkConfig(filepath.Join(dir, "nonexistent.yaml"))
	// checkConfig with an explicit path that doesn't exist: LoadConfig will fail.
	if check.Status == "ok" {
		t.Error("expected fail or warn when config file does not exist")
	}
	if cfg != nil {
		t.Error("expected nil config on error")
	}
}

func TestCheckConfig_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, `
branches:
  - name: ""
`)
	_, cfg, check := checkConfig(path)
	if check.Status != "fail" {
		t.Errorf("expected fail for invalid config, got %q: %s", check.Status, check.Message)
	}
	_ = cfg
}

func TestCheckPlugins_NotFound(t *testing.T) {
	cfg := &config.Config{
		Branches: []config.BranchConfig{{Name: "main"}},
		Plugins: []config.PluginConfig{
			{Uses: "semrel-plugin-definitely-not-installed-xyz123"},
		},
	}
	checks := checkPlugins(cfg)
	if len(checks) == 0 {
		t.Fatal("expected at least one check")
	}
	found := false
	for _, c := range checks {
		if c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one fail check for missing plugin")
	}
}

func TestCheckPlugins_ByPath_NotFound(t *testing.T) {
	cfg := &config.Config{
		Branches: []config.BranchConfig{{Name: "main"}},
		Plugins: []config.PluginConfig{
			{Path: "/definitely/does/not/exist/semrel-plugin-test"},
		},
	}
	checks := checkPlugins(cfg)
	// resolvePluginBinary with a non-existent path should fail.
	foundFail := false
	for _, c := range checks {
		if c.Status == "fail" {
			foundFail = true
		}
	}
	if !foundFail {
		t.Error("expected fail for plugin at non-existent path")
	}
}

func TestCheckPlugins_ByPath_Found(t *testing.T) {
	dir := t.TempDir()
	// Create a fake plugin binary file.
	binPath := filepath.Join(dir, "semrel-plugin-fake")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho {}"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Branches: []config.BranchConfig{{Name: "main"}},
		Plugins:  []config.PluginConfig{{Path: binPath}},
	}
	checks := checkPlugins(cfg)
	if len(checks) == 0 {
		t.Fatal("expected at least one check")
	}
	for _, c := range checks {
		if c.Status != "ok" {
			t.Errorf("expected ok, got %q: %s", c.Status, c.Message)
		}
	}
}

func TestCheckEnvVars_GitHubTokenMissing(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	cfg := &config.Config{
		Branches: []config.BranchConfig{{Name: "main"}},
		Plugins:  []config.PluginConfig{{Uses: "provider-github"}},
	}
	checks := checkEnvVars(cfg)
	found := false
	for _, c := range checks {
		if strings.Contains(c.Name, "github_token") {
			found = true
			if c.Status != "warn" {
				t.Errorf("expected warn for missing GITHUB_TOKEN, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Error("expected a check for GITHUB_TOKEN")
	}
}

func TestCheckEnvVars_GitHubTokenSet(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test_token")

	cfg := &config.Config{
		Branches: []config.BranchConfig{{Name: "main"}},
		Plugins:  []config.PluginConfig{{Uses: "provider-github"}},
	}
	checks := checkEnvVars(cfg)
	found := false
	for _, c := range checks {
		if strings.Contains(c.Name, "github_token") {
			found = true
			if c.Status != "ok" {
				t.Errorf("expected ok for set GITHUB_TOKEN, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Error("expected a check for GITHUB_TOKEN")
	}
}

func TestRunDoctor_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)

	// Redirect stdout by capturing via a pipe.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// runDoctor will fail the git-repository check (temp dir is not a repo) — that's fine.
	_ = runDoctor(t.Context(), cfgPath, "json", false)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result DoctorResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("could not parse JSON output: %v\nraw: %s", err, buf.String())
	}
	if len(result.Checks) == 0 {
		t.Error("expected non-empty checks in JSON output")
	}
}

func TestRunDoctor_HumanOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = runDoctor(t.Context(), cfgPath, "text", false)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, "config-file") {
		t.Errorf("expected config-file in output, got: %s", output)
	}
}

func TestDoctorCommand_RegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Use == "doctor" {
			return
		}
	}
	t.Error("expected 'doctor' command registered in root")
}
