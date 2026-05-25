// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package cargo provides a Rust Cargo crate publishing plugin.
// It updates the version field in Cargo.toml and publishes the crate
// to a registry (defaults to crates.io) using the cargo CLI.
package cargo

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// CargoToml holds the minimal fields read from a Cargo.toml file.
type CargoToml struct {
	// Name is the crate name.
	Name string
	// Version is the current crate version.
	Version string
}

// UpdateVersion reads a Cargo.toml file, updates the version in the [package]
// section, and writes it back. It preserves comments and all other fields.
func UpdateVersion(cargoTOMLPath, version string) (*CargoToml, error) {
	data, err := os.ReadFile(cargoTOMLPath)
	if err != nil {
		return nil, fmt.Errorf("cargo: read Cargo.toml: %w", err)
	}

	updated, meta, err := updateVersionInTOML(data, version)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(cargoTOMLPath, updated, 0o644); err != nil {
		return nil, fmt.Errorf("cargo: write Cargo.toml: %w", err)
	}
	return meta, nil
}

// updateVersionInTOML parses and updates the version field in Cargo.toml content.
func updateVersionInTOML(data []byte, version string) ([]byte, *CargoToml, error) {
	versionRe := regexp.MustCompile(`^(version\s*=\s*)"[^"]*"`)
	nameRe := regexp.MustCompile(`^(name\s*=\s*)"([^"]*)"`)

	var (
		lines       []string
		inPackage   bool
		versionSet  bool
		meta        CargoToml
	)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track which section we're in
		if strings.HasPrefix(trimmed, "[") {
			inPackage = trimmed == "[package]"
		}

		if inPackage {
			if m := nameRe.FindStringSubmatch(trimmed); m != nil {
				meta.Name = m[2]
			}
			if !versionSet && versionRe.MatchString(trimmed) {
				// Replace version value
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				line = indent + fmt.Sprintf(`version = "%s"`, version)
				versionSet = true
				meta.Version = version
			}
		}

		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("cargo: scan Cargo.toml: %w", err)
	}
	if !versionSet {
		return nil, nil, fmt.Errorf("cargo: version field not found in [package] section")
	}
	return []byte(strings.Join(lines, "\n")), &meta, nil
}

// Publisher publishes Rust crates using the cargo CLI.
type Publisher struct {
	cfg Config
}

// Config holds the publishing configuration.
type Config struct {
	// Token is the crates.io API token.
	Token string
	// Registry is the registry URL or name (defaults to crates.io).
	// Use a registry name defined in .cargo/config.toml or the full URL.
	Registry string
	// AllowDirty allows publishing with uncommitted changes (for testing).
	AllowDirty bool
	// DryRun performs a dry-run publish without actually uploading.
	DryRun bool
}

// NewPublisher creates a Publisher with the given configuration.
func NewPublisher(cfg Config) *Publisher {
	return &Publisher{cfg: cfg}
}

// Publish runs "cargo publish" in the given directory.
func (p *Publisher) Publish(ctx context.Context, crateDir string) error {
	args := []string{"publish"}
	if p.cfg.Registry != "" && p.cfg.Registry != "crates.io" {
		args = append(args, "--registry", p.cfg.Registry)
	}
	if p.cfg.AllowDirty {
		args = append(args, "--allow-dirty")
	}
	if p.cfg.DryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = crateDir
	env := os.Environ()
	if p.cfg.Token != "" {
		env = append(env, "CARGO_REGISTRY_TOKEN="+p.cfg.Token)
	}
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cargo: publish: %w\n%s", err, out)
	}
	return nil
}

// ReadVersion reads the version from a Cargo.toml file.
func ReadVersion(cargoTOMLPath string) (string, error) {
	data, err := os.ReadFile(cargoTOMLPath)
	if err != nil {
		return "", fmt.Errorf("cargo: read Cargo.toml: %w", err)
	}

	versionRe := regexp.MustCompile(`^version\s*=\s*"([^"]*)"`)
	inPackage := false

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inPackage = line == "[package]"
		}
		if inPackage {
			if m := versionRe.FindStringSubmatch(line); m != nil {
				return m[1], nil
			}
		}
	}
	return "", fmt.Errorf("cargo: version not found in Cargo.toml")
}

// IsCargoAvailable reports whether the cargo CLI is installed.
func IsCargoAvailable() bool {
	_, err := exec.LookPath("cargo")
	return err == nil
}
