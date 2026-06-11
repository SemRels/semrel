// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import (
	"fmt"
	"strings"
)

// PluginRegistry is the root document returned by the plugin registry.
type PluginRegistry struct {
	Plugins []PluginMeta `json:"plugins"`
}

// PluginMeta contains discovery metadata for a single plugin.
type PluginMeta struct {
	// Namespace groups plugins by origin, e.g. "@semrel" for the official org.
	// Omitted for community plugins that do not carry a namespace.
	Namespace   string          `json:"namespace,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	License     string          `json:"license"`
	Category    string          `json:"category"`
	Repository  string          `json:"repository,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Downloads   int64           `json:"downloads,omitempty"`
	Versions    []PluginVersion `json:"versions"`
}

// PluginVersion describes one downloadable plugin release.
type PluginVersion struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"releaseDate"`
	Changelog   string `json:"changelog,omitempty"`
	// DownloadURL is the fallback URL used when no platform-specific URL is available.
	DownloadURL string `json:"downloadUrl"`
	// DownloadURLs maps platform keys ("linux_amd64", "windows_amd64", etc.) to direct binary URLs.
	// When present, the correct URL is selected automatically based on the running OS/arch.
	DownloadURLs  map[string]string `json:"downloadUrls,omitempty"`
	Checksums     map[string]string `json:"checksums"`
	Compatibility *Compatibility    `json:"compatibility,omitempty"`
	Prerelease    bool              `json:"prerelease,omitempty"`
}

// Compatibility contains optional go-semrel/plugin compatibility hints.
type Compatibility struct {
	MinSemrelVersion string `json:"minSemrelVersion,omitempty"`
	GRPCVersion      string `json:"gRPCVersion,omitempty"`
}

// categoryPrefixes lists the category prefixes that .semrel.yaml configs may
// include in a plugin's "uses" field (e.g. "condition-github-actions",
// "provider-github", "updater-go"). The registry stores only the short name
// ("github-actions", "github", "go"), so FindPlugin strips these prefixes as a
// fallback when no exact match is found.
var categoryPrefixes = []string{
	"provider-", "condition-", "analyzer-", "generator-", "updater-", "hook-",
}

// FindPlugin returns the registry entry for the named plugin.
//
// The name argument may be:
//   - a bare name: "github" or "condition-github-actions"
//   - a category-prefixed name: "provider-github" (prefix is stripped on fallback)
//   - a namespaced ref: "@semrel/github"
//
// Lookup order:
//  1. Exact name match (respecting namespace when given).
//  2. Name with each category prefix stripped, in order.
//
// When a bare name is used, the first matching entry is returned regardless of
// namespace, which preserves backward compatibility with older plugins.json files
// that have no namespace field.
func (r *PluginRegistry) FindPlugin(name string) (*PluginMeta, error) {
	inputNS, bareName := splitPluginRef(name)

	if p := r.findByBareName(inputNS, bareName); p != nil {
		return p, nil
	}

	// Fallback: strip a single category prefix and try again.
	// This allows config entries like "provider-github" or "condition-github-actions"
	// to resolve registry entries stored under their short names ("github", "github-actions").
	for _, prefix := range categoryPrefixes {
		if strings.HasPrefix(strings.ToLower(bareName), prefix) {
			short := bareName[len(prefix):]
			if p := r.findByBareName(inputNS, short); p != nil {
				return p, nil
			}
		}
	}

	return nil, newRegistryError(ErrCodeNotFound, fmt.Sprintf("plugin %q not found", name), nil)
}

// findByBareName returns the first plugin whose Name matches bareName (case-insensitive).
// When inputNS is non-empty, the plugin namespace must also match.
// (see below)"
// matches a registry entry that stores the namespace as "SemRels" (and vice-versa).
func (r *PluginRegistry) findByBareName(inputNS, bareName string) *PluginMeta {
	for i := range r.Plugins {
		p := &r.Plugins[i]
		if !strings.EqualFold(p.Name, bareName) {
			continue
		}
		if inputNS != "" {
			// Normalize both sides: strip leading "@" before comparing.
			pNS := strings.TrimPrefix(p.Namespace, "@")
			reqNS := strings.TrimPrefix(inputNS, "@")
			if !strings.EqualFold(pNS, reqNS) {
				continue
			}
		}
		return p
	}
	return nil
}

// splitPluginRef splits "@namespace/name" into (namespace, name).
// For bare names without a leading "@", it returns ("", name).
func splitPluginRef(ref string) (namespace, name string) {
	if strings.HasPrefix(ref, "@") {
		if slash := strings.Index(ref, "/"); slash > 0 {
			return ref[:slash], ref[slash+1:]
		}
	}
	return "", ref
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

// DownloadURLForPlatform returns the best download URL for the given OS/arch.
// Prefers a platform-specific URL from DownloadURLs; falls back to DownloadURL.
func (v *PluginVersion) DownloadURLForPlatform(goos, goarch string) string {
	if url, ok := v.DownloadURLs[platformKey(goos, goarch)]; ok && url != "" {
		return url
	}
	return v.DownloadURL
}

func platformKey(goos, goarch string) string {
	return goos + "_" + goarch
}
