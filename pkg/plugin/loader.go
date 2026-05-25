// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package plugin provides the plugin interface and loader.
// See: https://github.com/SemRels/semrel/issues/8
package plugin

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/GoSemantics/semrel/internal/registry"
)

// Plugin is the interface for semrel plugins.
// Plugins are loaded as gRPC services.
// Issue: https://github.com/SemRels/semrel/issues/8
type Plugin interface {
	Name() string
	Version() string
	PreRelease(ctx context.Context, config map[string]interface{}) error
	PostRelease(ctx context.Context, config map[string]interface{}) error
}

// Loader resolves plugin binaries from local discovery and the remote registry.
type Loader struct {
	registryClient *registry.RegistryClient
	initErr        error
}

// NewLoader creates a new plugin loader using environment-based registry configuration.
func NewLoader() *Loader {
	client, err := registry.NewRegistryClientFromEnv()
	return &Loader{registryClient: client, initErr: err}
}

// NewLoaderWithRegistryClient creates a loader with an explicit registry client.
func NewLoaderWithRegistryClient(client *registry.RegistryClient) *Loader {
	return &Loader{registryClient: client}
}

// FetchMetadata returns the cached or remote plugin registry metadata.
func (l *Loader) FetchMetadata(ctx context.Context) (*registry.PluginRegistry, error) {
	if err := l.ensureRegistryClient(); err != nil {
		return nil, err
	}
	return l.registryClient.FetchMetadata(ctx)
}

// ResolvePluginBinary resolves a plugin for the current runtime platform.
func (l *Loader) ResolvePluginBinary(ctx context.Context, name, version string) (string, error) {
	return l.ResolvePluginBinaryForPlatform(ctx, name, version, runtime.GOOS, runtime.GOARCH)
}

// ResolvePluginBinaryForPlatform resolves a plugin locally or downloads it from the registry.
func (l *Loader) ResolvePluginBinaryForPlatform(ctx context.Context, name, version, goos, goarch string) (string, error) {
	if err := l.ensureRegistryClient(); err != nil {
		return "", err
	}
	return l.registryClient.GetPlugin(ctx, name, version, goos, goarch)
}

// ValidateChecksum validates a plugin binary before it is used.
func (l *Loader) ValidateChecksum(filePath, expected string) error {
	if err := l.ensureRegistryClient(); err != nil {
		return err
	}
	return l.registryClient.ValidateChecksum(filePath, expected)
}

// LoadPlugins discovers and loads plugins from .semrel/.
func (l *Loader) LoadPlugins(dir string) ([]Plugin, error) {
	return nil, fmt.Errorf("plugin loading from %s is not implemented yet", dir)
}

func (l *Loader) ensureRegistryClient() error {
	if l.initErr != nil {
		return l.initErr
	}
	if l.registryClient == nil {
		return errors.New("registry client is not configured")
	}
	return nil
}
