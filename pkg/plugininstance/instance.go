// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package plugininstance provides support for running the same plugin binary
// multiple times with different configuration instances (named instances).
//
// This is used when, for example, you want to publish to two different
// registries using the same plugin but with different credentials or targets:
//
//	plugins:
//	  - uses: semrel-plugin-publish
//	    name: publish-staging
//	    config:
//	      registry: staging.registry.example.com
//	  - uses: semrel-plugin-publish
//	    name: publish-prod
//	    config:
//	      registry: prod.registry.example.com
//
// Each instance has an independent name, config, and execution environment.
// Plugin binaries are resolved once and reused across instances of the same
// plugin type.
//
// See: https://github.com/SemRels/semrel/issues/44
package plugininstance

import (
	"context"
	"fmt"
)

// PluginSpec describes a single plugin instance from the semrel config.
type PluginSpec struct {
	// Uses is the plugin name / semver reference (e.g. "semrel-plugin-github@1.2.0").
	Uses string
	// Path is the local binary path (mutually exclusive with Uses).
	Path string
	// Name is an optional human-readable instance name.
	// Defaults to Uses or Path if not set.
	Name string
	// Config is the instance-specific configuration map.
	Config map[string]interface{}
}

// InstanceID returns a unique identifier for the plugin instance.
// If Name is set it is used; otherwise Uses or Path is used.
func (s PluginSpec) InstanceID() string {
	if s.Name != "" {
		return s.Name
	}
	if s.Uses != "" {
		return s.Uses
	}
	return s.Path
}

// Runner is a function type that executes a single plugin instance.
// Implementations call PreRelease or PostRelease on the underlying plugin binary.
type Runner func(ctx context.Context, spec PluginSpec) error

// Orchestrator runs a list of plugin specs in declaration order.
// Each spec is an independent instance — the same binary can appear multiple
// times with different configs.
type Orchestrator struct {
	runner Runner
}

// NewOrchestrator creates an orchestrator using the provided runner function.
// In production the runner resolves and executes the plugin binary; in tests
// a mock runner can be injected.
func NewOrchestrator(runner Runner) *Orchestrator {
	return &Orchestrator{runner: runner}
}

// Run executes all plugin instances in order.
// Errors from any instance are collected; execution continues so that all
// instances are attempted. The combined error is returned if any failed.
func (o *Orchestrator) Run(ctx context.Context, specs []PluginSpec) error {
	var errs []error
	for _, spec := range specs {
		if err := o.runner(ctx, spec); err != nil {
			errs = append(errs, fmt.Errorf("plugin %q: %w", spec.InstanceID(), err))
		}
	}
	return joinErrors(errs)
}

// joinErrors concatenates multiple errors into a single error.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	msg := errs[0].Error()
	for _, e := range errs[1:] {
		msg += "; " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
