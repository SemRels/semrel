// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package builtins_test

import (
	"context"
	"testing"

	"github.com/GoSemantics/semrel/pkg/builtins"
	"github.com/GoSemantics/semrel/pkg/plugin"
)

// expectedPlugins lists all plugin names that must be registered in DefaultRegistry.
var expectedPlugins = []string{
	"github", "npm", "docker", "helm",
	"slack", "matrix", "gitlab", "gitea",
	"cargo", "python", "gradle", "maven",
	"gobinary",
}

func TestDefaultRegistry_RegistersAllBuiltins(t *testing.T) {
	reg := builtins.DefaultRegistry()
	for _, name := range expectedPlugins {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("DefaultRegistry: plugin %q not found", name)
		}
	}
}

func TestDefaultRegistry_CountMatchesExpected(t *testing.T) {
	reg := builtins.DefaultRegistry()
	got := len(reg.List())
	want := len(expectedPlugins)
	if got != want {
		t.Errorf("DefaultRegistry: expected %d plugins, got %d", want, got)
	}
}

func TestDefaultRegistry_NoDuplicates(t *testing.T) {
	reg := builtins.DefaultRegistry()
	seen := make(map[string]bool)
	for _, name := range reg.List() {
		if seen[name] {
			t.Errorf("duplicate plugin registered: %q", name)
		}
		seen[name] = true
	}
}

func TestDryRun_AllPluginsSkipExecution(t *testing.T) {
	ctx := context.Background()
	reg := builtins.DefaultRegistry()
	rel := plugin.ReleaseContext{
		Version:  "1.2.3",
		TagName:  "v1.2.3",
		IsDryRun: true,
	}

	for _, name := range expectedPlugins {
		t.Run(name, func(t *testing.T) {
			p, ok := reg.Get(name)
			if !ok {
				t.Fatalf("plugin %q not found", name)
			}
			result, err := p.Execute(ctx, rel)
			if err != nil {
				t.Fatalf("plugin %q dry-run returned error: %v", name, err)
			}
			if result == nil {
				t.Fatalf("plugin %q dry-run returned nil result", name)
			}
			if result.Skipped {
				t.Errorf("plugin %q: dry-run should return success result, not skipped", name)
			}
			if v := result.Outputs["dry_run"]; v != "true" {
				t.Errorf("plugin %q: expected dry_run=true in outputs, got %q", name, v)
			}
		})
	}
}

func TestDockerPlugin_SkipWhenNoImage(t *testing.T) {
	ctx := context.Background()
	reg := builtins.DefaultRegistry()

	p, ok := reg.Get("docker")
	if !ok {
		t.Fatal("docker plugin not found")
	}

	// No DOCKER_IMAGE env and no metadata → should skip gracefully
	rel := plugin.ReleaseContext{
		Version:  "1.0.0",
		TagName:  "v1.0.0",
		IsDryRun: false,
		Metadata: nil,
	}
	result, err := p.Execute(ctx, rel)
	if err != nil {
		t.Fatalf("docker plugin: unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("docker plugin: returned nil result")
	}
	if !result.Skipped {
		t.Errorf("docker plugin: expected Skipped=true when no image configured, got false")
	}
}

func TestRegistry_Execute_UnknownPlugin(t *testing.T) {
	reg := builtins.DefaultRegistry()
	_, err := reg.Execute("nonexistent", context.Background(), plugin.ReleaseContext{IsDryRun: true})
	if err == nil {
		t.Error("expected error for unknown plugin, got nil")
	}
}

func TestArgsToMeta_Roundtrip(t *testing.T) {
	ctx := context.Background()
	reg := builtins.DefaultRegistry()

	p, ok := reg.Get("docker")
	if !ok {
		t.Fatal("docker plugin not found")
	}

	// Metadata image key → should skip (IsDockerAvailable false in CI) but image is set
	rel := plugin.ReleaseContext{
		Version:  "2.0.0",
		TagName:  "v2.0.0",
		IsDryRun: true,
		Metadata: map[string]string{"image": "myrepo/myapp"},
	}
	result, err := p.Execute(ctx, rel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	// dry-run so should succeed
	if result.Outputs["dry_run"] != "true" {
		t.Errorf("expected dry_run=true, got %q", result.Outputs["dry_run"])
	}
}
