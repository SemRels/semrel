// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package terraform provides a built-in plugin for publishing providers and
// modules to the Terraform/OpenTofu registry.
//
// The Terraform Registry uses GPG-signed checksums to verify provider packages.
// Publishing a new provider version involves:
//  1. Uploading binaries to GitHub Releases (done by semrel core)
//  2. Notifying registry.terraform.io (or a private registry) about the release
//  3. Providing a checksums file signed with a GPG key
//
// For modules, publishing is simpler: tag the repository in the correct format
// and the registry polls GitHub automatically. This package provides helpers
// to generate the required file manifests.
//
// See: https://github.com/SemRels/semrel/issues/32
package terraform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ProviderPlatform identifies a provider binary for a specific OS/arch combination.
type ProviderPlatform struct {
	// OS is the target operating system (e.g. "linux", "darwin", "windows").
	OS string
	// Arch is the target architecture (e.g. "amd64", "arm64").
	Arch string
	// BinaryPath is the path to the compiled provider binary .zip file.
	BinaryPath string
}

// ProviderManifest is the JSON manifest used by Terraform registries to
// describe all binaries in a provider release.
type ProviderManifest struct {
	// Version is the provider version (without 'v' prefix).
	Version string `json:"version"`
	// Protocols lists the Terraform plugin protocol versions supported.
	Protocols []string `json:"protocols"`
	// Platforms lists each OS/arch binary with its filename, SHA256, and download URL.
	Platforms []ProviderPlatformEntry `json:"platforms"`
}

// ProviderPlatformEntry describes a single OS/arch provider binary.
type ProviderPlatformEntry struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	SHA256      string `json:"shasum"`
}

// ManifestConfig configures a provider manifest generation run.
type ManifestConfig struct {
	// Namespace is the registry namespace (e.g. "hashicorp").
	Namespace string
	// Type is the provider type name (e.g. "aws").
	Type string
	// Version is the provider version (without 'v' prefix).
	Version string
	// Protocols is the list of supported Terraform plugin protocols.
	// Defaults to ["5.0"] for providers built with the SDK.
	Protocols []string
	// DownloadURLTemplate is the download URL with {filename} placeholder.
	DownloadURLTemplate string
}

// ManifestGenerator generates Terraform provider release manifests.
type ManifestGenerator struct {
	cfg ManifestConfig
}

// NewManifestGenerator creates a generator with the given configuration.
func NewManifestGenerator(cfg ManifestConfig) *ManifestGenerator {
	if len(cfg.Protocols) == 0 {
		cfg.Protocols = []string{"5.0"}
	}
	return &ManifestGenerator{cfg: cfg}
}

// GenerateManifest computes SHA-256 for each platform binary and returns
// the ProviderManifest.
func (g *ManifestGenerator) GenerateManifest(platforms []ProviderPlatform) (*ProviderManifest, error) {
	manifest := &ProviderManifest{
		Version:   g.cfg.Version,
		Protocols: g.cfg.Protocols,
	}
	for _, p := range platforms {
		digest, err := sha256File(p.BinaryPath)
		if err != nil {
			return nil, fmt.Errorf("terraform: sha256 %s: %w", p.BinaryPath, err)
		}
		filename := filepath.Base(p.BinaryPath)
		url := strings.ReplaceAll(g.cfg.DownloadURLTemplate, "{filename}", filename)
		manifest.Platforms = append(manifest.Platforms, ProviderPlatformEntry{
			OS:          p.OS,
			Arch:        p.Arch,
			Filename:    filename,
			DownloadURL: url,
			SHA256:      digest,
		})
	}
	return manifest, nil
}

// MarshalManifest serializes a ProviderManifest to indented JSON.
func MarshalManifest(m *ProviderManifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// WriteChecksums writes a SHA256SUMS-style file compatible with Terraform
// registry expectations. Each line is "<sha256>  <filename>".
func WriteChecksums(w io.Writer, manifest *ProviderManifest) error {
	for _, p := range manifest.Platforms {
		if _, err := fmt.Fprintf(w, "%s  %s\n", p.SHA256, p.Filename); err != nil {
			return err
		}
	}
	return nil
}

// ModuleTagName returns the git tag name for a Terraform module release.
// The Terraform registry requires tags in the format "<major>.<minor>.<patch>"
// (without 'v' prefix) for modules.
func ModuleTagName(version string) string {
	return strings.TrimPrefix(version, "v")
}

// ProviderTagName returns the git tag name for a Terraform provider release.
// Providers use the standard "v<semver>" format.
func ProviderTagName(version string) string {
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// RegistryURL returns the public URL for a provider on registry.terraform.io.
func RegistryURL(namespace, providerType, version string) string {
	return fmt.Sprintf("https://registry.terraform.io/providers/%s/%s/%s", namespace, providerType, version)
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
