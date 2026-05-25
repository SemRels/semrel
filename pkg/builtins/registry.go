// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package builtins provides the default (empty) plugin registry for semrel.
// Plugin implementations are NOT bundled into the core binary.
// Install plugins separately via: semrel plugin install <name>
// See: https://semrels.github.io/semrel-registry for available plugins.
package builtins

import "github.com/GoSemantics/semrel/pkg/plugin"

// DefaultRegistry returns an empty plugin registry.
// No built-in plugin implementations are bundled in the core.
// Plugins must be installed separately.
func DefaultRegistry() *plugin.Registry {
	return plugin.NewRegistry()
}
