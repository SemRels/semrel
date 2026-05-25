// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package helm_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoSemantics/semrel/pkg/helm"
	"github.com/GoSemantics/semrel/pkg/plugin"
)

func writePluginChartYAML(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "Chart.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write Chart.yaml: %v", err)
	}
	return p
}

const sampleChart = `apiVersion: v2
name: my-app
description: A sample Helm chart
version: 0.1.0
appVersion: 0.1.0
`

func TestHelmPlugin_Name(t *testing.T) {
	p := helm.NewPlugin(helm.PluginConfig{})
	if got := p.Name(); got != "helm" {
		t.Errorf("Name() = %q, want %q", got, "helm")
	}
}

func TestHelmPlugin_Version(t *testing.T) {
	p := helm.NewPlugin(helm.PluginConfig{})
	if got := p.Version(); got == "" {
		t.Error("Version() should not be empty")
	}
}

func TestHelmPlugin_Validate_NoChart(t *testing.T) {
	p := helm.NewPlugin(helm.PluginConfig{ChartDir: "/nonexistent/path"})
	if err := p.Validate(); err == nil {
		t.Error("Validate() should fail when Chart.yaml does not exist")
	}
}

func TestHelmPlugin_Validate_OK(t *testing.T) {
	dir := t.TempDir()
	writePluginChartYAML(t, dir, sampleChart)
	p := helm.NewPlugin(helm.PluginConfig{ChartDir: dir})
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestHelmPlugin_Validate_PublishWithoutPublisher(t *testing.T) {
	dir := t.TempDir()
	writePluginChartYAML(t, dir, sampleChart)
	p := helm.NewPlugin(helm.PluginConfig{ChartDir: dir, Publish: true, Publisher: nil})
	if err := p.Validate(); err == nil {
		t.Error("Validate() should fail when Publish=true but no Publisher configured")
	}
}

func TestHelmPlugin_Execute_DryRun(t *testing.T) {
	dir := t.TempDir()
	writePluginChartYAML(t, dir, sampleChart)
	p := helm.NewPlugin(helm.PluginConfig{ChartDir: dir})

	rel := plugin.ReleaseContext{Version: "1.2.3", IsDryRun: true}
	result, err := p.Execute(context.Background(), rel)
	if err != nil {
		t.Fatalf("Execute() dry run error: %v", err)
	}
	if !result.Skipped && result.Outputs["dry_run"] != "true" {
		t.Errorf("Expected dry_run=true in outputs, got: %v", result.Outputs)
	}
}

func TestHelmPlugin_Execute_UpdatesVersion(t *testing.T) {
	dir := t.TempDir()
	writePluginChartYAML(t, dir, sampleChart)
	p := helm.NewPlugin(helm.PluginConfig{ChartDir: dir, UpdateAppVersion: true})

	rel := plugin.ReleaseContext{Version: "2.0.0"}
	result, err := p.Execute(context.Background(), rel)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Outputs["chart_version"] != "2.0.0" {
		t.Errorf("chart_version = %q, want %q", result.Outputs["chart_version"], "2.0.0")
	}
	if result.Outputs["app_version"] != "2.0.0" {
		t.Errorf("app_version = %q, want %q", result.Outputs["app_version"], "2.0.0")
	}

	// Verify the file was actually updated
	meta, err := helm.ReadChartMeta(filepath.Join(dir, "Chart.yaml"))
	if err != nil {
		t.Fatalf("ReadChartMeta error: %v", err)
	}
	if meta.Version != "2.0.0" {
		t.Errorf("Chart.yaml version = %q, want %q", meta.Version, "2.0.0")
	}
}

func TestHelmPlugin_Execute_OutputsChartName(t *testing.T) {
	dir := t.TempDir()
	writePluginChartYAML(t, dir, sampleChart)
	p := helm.NewPlugin(helm.PluginConfig{ChartDir: dir})

	rel := plugin.ReleaseContext{Version: "3.1.0"}
	result, err := p.Execute(context.Background(), rel)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Outputs["chart_name"] != "my-app" {
		t.Errorf("chart_name = %q, want %q", result.Outputs["chart_name"], "my-app")
	}
}
