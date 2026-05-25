// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package helm

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/GoSemantics/semrel/pkg/plugin"
)

// Plugin is a semrel Executor plugin that updates a Helm Chart.yaml version
// and optionally packages + pushes the chart to a registry on release.
type Plugin struct {
	plugin.BasePlugin
	cfg PluginConfig
}

// PluginConfig configures the Helm plugin.
type PluginConfig struct {
	// ChartDir is the path to the Helm chart directory (containing Chart.yaml).
	// Defaults to "chart" if empty.
	ChartDir string
	// UpdateAppVersion controls whether to also set appVersion to the release version.
	UpdateAppVersion bool
	// Publish controls whether to package and push the chart after version update.
	Publish bool
	// Publisher is the chart publisher used when Publish is true.
	Publisher *Publisher
}

// NewPlugin creates a Helm executor plugin.
func NewPlugin(cfg PluginConfig) *Plugin {
	if cfg.ChartDir == "" {
		cfg.ChartDir = "chart"
	}
	return &Plugin{
		BasePlugin: plugin.NewBasePlugin("helm", "1.0.0"),
		cfg:        cfg,
	}
}

// Validate checks that the chart directory contains a Chart.yaml file.
func (p *Plugin) Validate() error {
	chartPath := filepath.Join(p.cfg.ChartDir, "Chart.yaml")
	meta, err := ReadChartMeta(chartPath)
	if err != nil {
		return plugin.ErrInvalidConfig{
			Plugin:  p.Name(),
			Message: fmt.Sprintf("cannot read Chart.yaml at %s: %v", chartPath, err),
		}
	}
	if meta.Name == "" {
		return plugin.ErrInvalidConfig{
			Plugin:  p.Name(),
			Message: "Chart.yaml is missing the 'name' field",
		}
	}
	if p.cfg.Publish && p.cfg.Publisher == nil {
		return plugin.ErrInvalidConfig{
			Plugin:  p.Name(),
			Message: "publish is enabled but no Publisher is configured",
		}
	}
	return nil
}

// Execute updates the chart version and, if configured, packages and pushes the chart.
func (p *Plugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
	if rel.IsDryRun {
		return plugin.SuccessResult(p.Name(), map[string]string{
			"dry_run": "true",
			"version": rel.Version,
		}), nil
	}

	chartPath := filepath.Join(p.cfg.ChartDir, "Chart.yaml")
	version := rel.Version

	appVersion := ""
	if p.cfg.UpdateAppVersion {
		appVersion = version
	}

	meta, err := UpdateChartVersion(chartPath, version, appVersion)
	if err != nil {
		return nil, fmt.Errorf("helm plugin: update chart version: %w", err)
	}

	outputs := map[string]string{
		"chart_name":    meta.Name,
		"chart_version": meta.Version,
	}
	if meta.AppVersion != "" {
		outputs["app_version"] = meta.AppVersion
	}

	if p.cfg.Publish && p.cfg.Publisher != nil {
		tgz, err := p.cfg.Publisher.Package(ctx, p.cfg.ChartDir, "")
		if err != nil {
			return nil, fmt.Errorf("helm plugin: package chart: %w", err)
		}
		outputs["chart_tgz"] = tgz
	}

	return plugin.SuccessResult(p.Name(), outputs), nil
}
