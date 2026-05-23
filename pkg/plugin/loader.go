// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package plugin provides the plugin interface and loader.
// See: https://github.com/GoSemantics/semrel/issues/8
package plugin

import "context"

// Plugin is the interface for semrel plugins.
// Plugins are loaded as gRPC services.
// Issue: https://github.com/GoSemantics/semrel/issues/8
type Plugin interface {
Name() string
Version() string
PreRelease(ctx context.Context, config map[string]interface{}) error
PostRelease(ctx context.Context, config map[string]interface{}) error
}

// Loader loads plugins from the .semrel/ directory.
type Loader struct {
// Plugin discovery and loading
}

// NewLoader creates a new plugin loader.
func NewLoader() *Loader {
return &Loader{}
}

// LoadPlugins discovers and loads plugins from .semrel/
func (l *Loader) LoadPlugins(dir string) ([]Plugin, error) {
// TODO: Implement plugin discovery
panic("not implemented")
}
