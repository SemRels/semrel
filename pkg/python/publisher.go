// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package python provides Python package versioning and PyPI publishing.
// It updates version references in pyproject.toml (PEP 621/Poetry) and
// setup.cfg/setup.py, and publishes packages to PyPI using twine or the
// PyPA build + upload workflow.
package python

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// PyprojectToml holds the minimal fields from a pyproject.toml.
type PyprojectToml struct {
	// Name is the package name.
	Name string
	// Version is the current package version.
	Version string
}

// UpdatePyprojectVersion reads pyproject.toml, updates the version in [project]
// or [tool.poetry] section, and writes the file back.
func UpdatePyprojectVersion(path, version string) (*PyprojectToml, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("python: read pyproject.toml: %w", err)
	}

	updated, meta, err := updatePyprojectTOML(data, version)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return nil, fmt.Errorf("python: write pyproject.toml: %w", err)
	}
	return meta, nil
}

func updatePyprojectTOML(data []byte, version string) ([]byte, *PyprojectToml, error) {
	versionRe := regexp.MustCompile(`^(version\s*=\s*)"[^"]*"`)
	nameRe := regexp.MustCompile(`^(name\s*=\s*)"([^"]*)"`)

	var (
		lines      []string
		inSection  bool
		versionSet bool
		meta       PyprojectToml
	)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") {
			// Match [project] or [tool.poetry] sections
			section := strings.ToLower(trimmed)
			inSection = section == "[project]" || section == "[tool.poetry]"
		}

		if inSection {
			if m := nameRe.FindStringSubmatch(trimmed); m != nil {
				meta.Name = m[2]
			}
			if !versionSet && versionRe.MatchString(trimmed) {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				line = indent + fmt.Sprintf(`version = "%s"`, version)
				versionSet = true
				meta.Version = version
			}
		}

		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("python: scan pyproject.toml: %w", err)
	}
	if !versionSet {
		return nil, nil, fmt.Errorf("python: version field not found in pyproject.toml")
	}
	return []byte(strings.Join(lines, "\n")), &meta, nil
}

// UpdateSetupCfgVersion reads setup.cfg, updates the version field in [metadata],
// and writes the file back.
func UpdateSetupCfgVersion(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("python: read setup.cfg: %w", err)
	}

	versionRe := regexp.MustCompile(`^(version\s*=\s*)\S+`)
	inMetadata := false
	var lines []string
	versionSet := false

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") {
			inMetadata = strings.ToLower(trimmed) == "[metadata]"
		}

		if inMetadata && !versionSet && versionRe.MatchString(trimmed) {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			line = indent + "version = " + version
			versionSet = true
		}
		lines = append(lines, line)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// Publisher publishes Python packages to PyPI (or a custom index).
type Publisher struct {
	cfg Config
}

// Config holds the PyPI publishing configuration.
type Config struct {
	// Repository is the PyPI repository URL (defaults to https://upload.pypi.org/legacy/).
	Repository string
	// Username is the PyPI username (use "__token__" with API tokens).
	Username string
	// Password is the PyPI password or API token.
	Password string
	// SkipExisting skips upload if the version already exists (--skip-existing).
	SkipExisting bool
}

// NewPublisher creates a Publisher with the given configuration.
func NewPublisher(cfg Config) *Publisher {
	if cfg.Repository == "" {
		cfg.Repository = "https://upload.pypi.org/legacy/"
	}
	return &Publisher{cfg: cfg}
}

// Build runs the Python build system (python -m build) to create dist/ artifacts.
func (p *Publisher) Build(ctx context.Context, packageDir string) error {
	cmd := exec.CommandContext(ctx, "python", "-m", "build")
	cmd.Dir = packageDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("python: build: %w\n%s", err, out)
	}
	return nil
}

// UploadWithTwine runs twine upload to publish dist artifacts.
func (p *Publisher) UploadWithTwine(ctx context.Context, packageDir, distGlob string) error {
	if distGlob == "" {
		distGlob = "dist/*"
	}
	args := []string{"upload", "--repository-url", p.cfg.Repository}
	if p.cfg.SkipExisting {
		args = append(args, "--skip-existing")
	}
	args = append(args, distGlob)

	cmd := exec.CommandContext(ctx, "twine", args...)
	cmd.Dir = packageDir
	env := os.Environ()
	if p.cfg.Username != "" {
		env = append(env, "TWINE_USERNAME="+p.cfg.Username)
	}
	if p.cfg.Password != "" {
		env = append(env, "TWINE_PASSWORD="+p.cfg.Password)
	}
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("python: twine upload: %w\n%s", err, out)
	}
	return nil
}

// IsTwineAvailable reports whether twine is installed.
func IsTwineAvailable() bool {
	_, err := exec.LookPath("twine")
	return err == nil
}

// IsPythonAvailable reports whether python is installed.
func IsPythonAvailable() bool {
	_, err := exec.LookPath("python")
	if err != nil {
		_, err = exec.LookPath("python3")
	}
	return err == nil
}
