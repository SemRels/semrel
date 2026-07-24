// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PluginRegistry is the root document returned by the plugin registry.
type PluginRegistry struct {
	SchemaVersion int          `json:"schemaVersion,omitempty"`
	GeneratedAt   string       `json:"generatedAt,omitempty"`
	Plugins       []PluginMeta `json:"plugins"`
}

// PluginMeta contains discovery metadata for a single plugin.
type PluginMeta struct {
	// ID is the stable resolver/schema/cache identifier used by transitional
	// registry documents. PackageName is the external canonical identity.
	ID          string `json:"id,omitempty"`
	PackageName string `json:"packageName,omitempty"`
	// Namespace groups plugins by origin, e.g. "@semrel" for the official org.
	// Omitted for community plugins that do not carry a namespace.
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	License     string `json:"license"`
	Category    string `json:"category"`
	Repository  string `json:"repository,omitempty"`
	// Aliases contains explicit legacy references accepted for this plugin.
	Aliases       PluginAliases `json:"aliases,omitempty"`
	LegacyAliases []string      `json:"legacyAliases,omitempty"`
	// ArtifactName is the stable cache and executable basename. It is separate
	// from the registry package identity so package renames do not break caches.
	ArtifactName string          `json:"artifactName,omitempty"`
	BinaryName   string          `json:"binaryName,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	Downloads    int64           `json:"downloads,omitempty"`
	Versions     []PluginVersion `json:"versions"`
}

// PluginAliases accepts both the legacy string form and transition metadata
// objects such as {"value":"teams","type":"legacy-id","pluginType":"hook"}.
type PluginAliases []string

// UnmarshalJSON accepts aliases encoded as strings or transition metadata objects.
func (a *PluginAliases) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*a = (*a)[:0]
	for _, raw := range values {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			*a = append(*a, value)
			continue
		}
		var alias struct {
			Value string `json:"value"`
			Ref   string `json:"ref"`
		}
		if err := json.Unmarshal(raw, &alias); err != nil {
			return fmt.Errorf("parse plugin alias: %w", err)
		}
		value = alias.Value
		if strings.TrimSpace(value) == "" {
			value = alias.Ref
		}
		if strings.TrimSpace(value) != "" {
			*a = append(*a, value)
		}
	}
	return nil
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

// FindPlugin returns the registry entry for the named plugin.
func (r *PluginRegistry) FindPlugin(name string) (*PluginMeta, error) {
	requested := strings.ToLower(strings.TrimSpace(name))
	if requested == "" {
		return nil, newRegistryError(ErrCodeNotFound, "plugin name is empty", nil)
	}

	// Historical aliases are resolved to a fixed canonical target before
	// inspecting metadata. This prevents collisions from becoming first-match.
	if canonical, ok := CanonicalLegacyRef(requested); ok {
		var matches []*PluginMeta
		canonicalName := canonical[strings.LastIndex(canonical, "/")+1:]
		for i := range r.Plugins {
			legacyCanonical, legacyKnown := CanonicalLegacyRef(r.Plugins[i].Name)
			if strings.EqualFold(r.Plugins[i].CanonicalRef(), canonical) ||
				strings.EqualFold(strings.TrimSpace(r.Plugins[i].Name), canonicalName) ||
				(normalizeCategory(r.Plugins[i].Category) == "" && legacyKnown &&
					strings.EqualFold(legacyCanonical, canonical)) {
				matches = append(matches, &r.Plugins[i])
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			return nil, newRegistryError(ErrCodeInvalidMetadata,
				fmt.Sprintf("canonical target %q is duplicated in registry metadata", canonical), nil)
		}
		return nil, newRegistryError(ErrCodeNotFound,
			fmt.Sprintf("historical alias %q targets unavailable plugin %q", name, canonical), nil)
	}

	var matches []*PluginMeta
	for i := range r.Plugins {
		p := &r.Plugins[i]
		if p.matchesRef(requested) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, newRegistryError(ErrCodeInvalidMetadata,
			fmt.Sprintf("plugin alias %q is ambiguous; use a canonical reference", name), nil)
	}
	return nil, newRegistryError(ErrCodeNotFound, fmt.Sprintf("plugin %q not found", name), nil)
}

func (p *PluginMeta) matchesRef(requested string) bool {
	for _, ref := range p.references() {
		if strings.EqualFold(strings.TrimSpace(ref), requested) {
			return true
		}
	}
	return false
}

func (p *PluginMeta) references() []string {
	refs := []string{p.CanonicalRef()}
	if p.IsFirstParty() {
		canonical := p.CanonicalRef()
		if slash := strings.LastIndex(canonical, "/"); slash >= 0 {
			refs = append(refs, canonical[slash+1:])
		}
		short := strings.ToLower(strings.TrimSpace(p.Name))
		refs = append(refs, short, firstPartyNamespace+"/"+short)
	}
	ns := normalizeNamespace(p.Namespace)
	if ns != "" {
		refs = append(refs, ns+"/"+p.Name)
	} else if !p.IsFirstParty() {
		refs = append(refs, p.Name)
	}
	refs = append(refs, p.Aliases...)
	refs = append(refs, p.LegacyAliases...)
	return refs
}

// IsFirstParty reports whether registry metadata identifies an official plugin.
func (p PluginMeta) IsFirstParty() bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.PackageName)), firstPartyNamespace+"/") {
		return true
	}
	if normalizeNamespace(p.Namespace) == firstPartyNamespace {
		return true
	}
	repository := strings.ToLower(strings.TrimSpace(p.Repository))
	return strings.Contains(repository, "github.com/semrels/")
}

// CanonicalRef returns the registry package identity. Legacy metadata stored
// first-party short names, so category is used to produce the typed identity.
func (p PluginMeta) CanonicalRef() string {
	if packageName := strings.ToLower(strings.TrimSpace(p.PackageName)); packageName != "" {
		if strings.HasPrefix(packageName, "@") {
			return packageName
		}
		if ns := normalizeNamespace(p.Namespace); ns != "" {
			return ns + "/" + packageName
		}
		return packageName
	}
	ns := normalizeNamespace(p.Namespace)
	name := strings.ToLower(strings.TrimSpace(p.Name))
	if p.IsFirstParty() {
		ns = firstPartyNamespace
		category := normalizeCategory(p.Category)
		if category != "" && !strings.HasPrefix(name, category+"-") {
			name = category + "-" + name
		}
	}
	if ns != "" {
		return ns + "/" + name
	}
	return name
}

// ExecutableName returns the stable artifact/cache basename, independently of
// the canonical package reference.
func (p PluginMeta) ExecutableName() string {
	name := strings.TrimSpace(p.ArtifactName)
	if name == "" {
		name = strings.TrimSpace(p.BinaryName)
	}
	name = strings.TrimPrefix(name, "semrel-plugin-")
	if name != "" {
		return name
	}
	if id := strings.TrimSpace(p.ID); id != "" {
		return id
	}

	name = strings.ToLower(strings.TrimSpace(p.Name))
	if p.IsFirstParty() {
		canonicalName := p.CanonicalRef()
		canonicalName = canonicalName[strings.LastIndex(canonicalName, "/")+1:]
		if legacy, ok := legacyArtifactNames[canonicalName]; ok {
			return legacy
		}
		return canonicalName
	}
	return name
}

// FindVersion returns the requested plugin version. If version is empty, the latest stable version is preferred.
//
// The registry stores versions in descending order (newest first). For the
// "latest" lookup we therefore iterate from the beginning of the slice.
func (p *PluginMeta) FindVersion(version string) (*PluginVersion, error) {
	if version == "" {
		// Iterate from the front (newest first in the registry's descending list).
		for i := range p.Versions {
			if !p.Versions[i].Prerelease {
				return &p.Versions[i], nil
			}
		}
		// All versions are pre-releases — return the newest (first entry).
		if len(p.Versions) > 0 {
			return &p.Versions[0], nil
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
