// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import "fmt"

// PluginRegistry is the root document returned by the plugin registry.
type PluginRegistry struct {
	Plugins []PluginMeta `json:"plugins"`
}

// PluginMeta contains discovery metadata for a single plugin.
type PluginMeta struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	License     string          `json:"license"`
	Category    string          `json:"category"`
	Repository  string          `json:"repository,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Versions    []PluginVersion `json:"versions"`
}

// PluginVersion describes one downloadable plugin release.
type PluginVersion struct {
	Version       string            `json:"version"`
	ReleaseDate   string            `json:"releaseDate"`
	Changelog     string            `json:"changelog,omitempty"`
	DownloadURL   string            `json:"downloadUrl"`
	Checksums     map[string]string `json:"checksums"`
	Compatibility *Compatibility    `json:"compatibility,omitempty"`
	Prerelease    bool              `json:"prerelease,omitempty"`
}

// Compatibility contains optional go-semrel/plugin compatibility hints.
type Compatibility struct {
	MinSemrelVersion string `json:"minSemrelVersion,omitempty"`
	GRPCVersion      string `json:"gRPCVersion,omitempty"`
}

// FindPlugin returns the registry entry for the named plugin.
func (r *PluginRegistry) FindPlugin(name string) (*PluginMeta, error) {
	for i := range r.Plugins {
		if r.Plugins[i].Name == name {
			return &r.Plugins[i], nil
		}
	}
	return nil, newRegistryError(ErrCodeNotFound, fmt.Sprintf("plugin %q not found", name), nil)
}

// FindVersion returns the requested plugin version. If version is empty, the latest stable version is preferred.
func (p *PluginMeta) FindVersion(version string) (*PluginVersion, error) {
	if version == "" {
		for i := len(p.Versions) - 1; i >= 0; i-- {
			if !p.Versions[i].Prerelease {
				return &p.Versions[i], nil
			}
		}
		if len(p.Versions) > 0 {
			return &p.Versions[len(p.Versions)-1], nil
		}
	}

	for i := range p.Versions {
		if p.Versions[i].Version == version {
			return &p.Versions[i], nil
		}
	}
	return nil, newRegistryError(ErrCodeNotFound, fmt.Sprintf("version %q not found for plugin %q", version, p.Name), nil)
}

// ChecksumForPlatform returns the checksum for the requested GOOS/GOARCH pair.
func (v *PluginVersion) ChecksumForPlatform(goos, goarch string) (string, error) {
	checksum, ok := v.Checksums[platformKey(goos, goarch)]
	if !ok || checksum == "" {
		return "", newRegistryError(ErrCodeNotFound, fmt.Sprintf("checksum for %s/%s not found", goos, goarch), nil)
	}
	return checksum, nil
}

func platformKey(goos, goarch string) string {
	return goos + "_" + goarch
}
