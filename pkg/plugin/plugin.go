// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package plugin defines lifecycle hooks, result types, and a Registry for
// semrel plugins. The base Plugin interface is defined in loader.go.
// This file adds the Executor interface, ReleaseContext, Result types,
// lifecycle hooks, and a local Registry for in-process plugins.
package plugin

import (
	"context"
	"fmt"
)

// ReleaseContext contains all information about the release being processed.
type ReleaseContext struct {
	// Version is the new semantic version (e.g., "v1.2.3").
	Version string
	// PreviousVersion is the previous semantic version.
	PreviousVersion string
	// TagName is the git tag name (may differ from Version, e.g., "myapp/v1.2.3").
	TagName string
	// Repository is the full repository name (e.g., "owner/repo").
	Repository string
	// Changelog is the generated release notes / changelog text.
	Changelog string
	// CommitSHA is the commit SHA the release is created from.
	CommitSHA string
	// IsPrerelease indicates whether this is a prerelease version.
	IsPrerelease bool
	// IsDryRun indicates that no actual changes should be made.
	IsDryRun bool
	// Metadata is a map of arbitrary key-value pairs for plugin-specific config.
	Metadata map[string]string
}

// Result contains the outcome of a plugin's execution.
type Result struct {
	// Name is the plugin name.
	Name string
	// Outputs is a map of key-value pairs produced by the plugin.
	Outputs map[string]string
	// Skipped indicates the plugin chose to skip execution.
	Skipped bool
	// SkipReason explains why the plugin was skipped.
	SkipReason string
}

// Executor is the extended plugin interface for in-process plugins that
// execute as part of the release pipeline. Differs from Plugin (loader.go)
// which handles gRPC-loaded external plugins.
type Executor interface {
	// Name returns the unique name of the plugin (e.g., "npm", "docker").
	Name() string

	// Version returns the plugin version string.
	Version() string

	// Validate checks the plugin configuration without performing any actions.
	// Returns an error if the configuration is invalid.
	Validate() error

	// Execute performs the plugin's main action during a release.
	Execute(ctx context.Context, rel ReleaseContext) (*Result, error)
}

// Lifecycle is an optional interface for plugins that need setup/teardown.
type Lifecycle interface {
	Executor

	// Setup is called before Execute to initialize resources.
	Setup(ctx context.Context) error

	// Teardown is called after Execute (even if it failed) to clean up.
	Teardown(ctx context.Context) error
}

// Analyzer is an optional interface for plugins that analyze commits to
// determine whether and how to bump the version.
type Analyzer interface {
	Executor

	// AnalyzeCommits analyzes a list of commit messages and returns the
	// recommended version bump ("major", "minor", "patch", or "none").
	AnalyzeCommits(ctx context.Context, commits []string) (string, error)
}

// BasePlugin provides default no-op implementations for optional interface methods.
// Embed this in your plugin to avoid implementing all methods.
type BasePlugin struct {
	name    string
	version string
}

// NewBasePlugin creates a BasePlugin with the given name and version.
func NewBasePlugin(name, version string) BasePlugin {
	return BasePlugin{name: name, version: version}
}

// Name returns the plugin name.
func (b BasePlugin) Name() string { return b.name }

// Version returns the plugin version.
func (b BasePlugin) Version() string { return b.version }

// Validate performs no validation by default (override to add checks).
func (b BasePlugin) Validate() error { return nil }

// SuccessResult returns a Result with the given outputs marked as successful.
func SuccessResult(name string, outputs map[string]string) *Result {
	if outputs == nil {
		outputs = make(map[string]string)
	}
	return &Result{Name: name, Outputs: outputs}
}

// SkippedResult returns a Result indicating the plugin was skipped.
func SkippedResult(name, reason string) *Result {
	return &Result{Name: name, Skipped: true, SkipReason: reason}
}

// ErrInvalidConfig is returned when plugin configuration is invalid.
type ErrInvalidConfig struct {
	Plugin  string
	Message string
}

func (e ErrInvalidConfig) Error() string {
	return fmt.Errorf("plugin %q: invalid config: %s", e.Plugin, e.Message).Error()
}

// Registry is a collection of registered in-process Executor plugins.
type Registry struct {
	executors map[string]Executor
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{executors: make(map[string]Executor)}
}

// Register adds a plugin to the registry. Returns an error if a plugin with
// the same name is already registered.
func (r *Registry) Register(p Executor) error {
	if _, exists := r.executors[p.Name()]; exists {
		return fmt.Errorf("plugin %q already registered", p.Name())
	}
	r.executors[p.Name()] = p
	return nil
}

// Get retrieves a plugin by name.
func (r *Registry) Get(name string) (Executor, bool) {
	p, ok := r.executors[name]
	return p, ok
}

// List returns all registered plugin names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.executors))
	for name := range r.executors {
		names = append(names, name)
	}
	return names
}

// RunAll executes all registered plugins in sequence.
// If a plugin returns an error, execution stops and the error is returned.
func (r *Registry) RunAll(ctx context.Context, rel ReleaseContext) ([]*Result, error) {
	var results []*Result
	for _, p := range r.executors {
		result, err := p.Execute(ctx, rel)
		if err != nil {
			return results, fmt.Errorf("plugin %q failed: %w", p.Name(), err)
		}
		results = append(results, result)
	}
	return results, nil
}

// ValidateAll validates all registered plugins.
func (r *Registry) ValidateAll() error {
	for _, p := range r.executors {
		if err := p.Validate(); err != nil {
			return err
		}
	}
	return nil
}
