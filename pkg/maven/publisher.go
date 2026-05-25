// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package maven provides a Maven build tool plugin for updating versions and
// publishing Java artifacts to Maven repositories.
package maven

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// POM represents the minimal fields of a Maven pom.xml.
type POM struct {
	XMLName    xml.Name `xml:"project"`
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Version    string   `xml:"version"`
}

// UpdatePOMVersion reads a pom.xml file, updates the <version> element in the
// root project, and writes the file back.
func UpdatePOMVersion(path, version string) (*POM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("maven: read pom.xml: %w", err)
	}

	// Parse current POM for metadata
	var pom POM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("maven: parse pom.xml: %w", err)
	}

	// Replace the first <version> element in the root project (not dependencies)
	updated, err := replaceRootVersion(data, version)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return nil, fmt.Errorf("maven: write pom.xml: %w", err)
	}

	pom.Version = version
	return &pom, nil
}

// replaceRootVersion replaces the first <version> tag in the root project element.
func replaceRootVersion(data []byte, version string) ([]byte, error) {
	// Find the first <version>...</version> outside of dependencies/parent
	// Strategy: find first occurrence at low nesting depth
	re := regexp.MustCompile(`(?s)(<project[^>]*>.*?<version>)[^<]*(</version>)`)
	if !re.Match(data) {
		return nil, fmt.Errorf("maven: <version> not found in root project element")
	}
	return re.ReplaceAll(data, []byte("${1}"+version+"${2}")), nil
}

// Publisher publishes Maven artifacts using the mvn CLI.
type Publisher struct {
	cfg Config
}

// Config holds Maven publishing configuration.
type Config struct {
	// SettingsXML is the path to a Maven settings.xml file (optional).
	SettingsXML string
	// Repository is the Maven repository URL for deployment.
	Repository string
	// RepositoryID is the server ID in settings.xml for authentication.
	RepositoryID string
	// Goals is the list of Maven goals/phases to execute (defaults to ["deploy"]).
	Goals []string
	// Profiles is the list of Maven profiles to activate.
	Profiles []string
	// SkipTests skips test execution during publishing.
	SkipTests bool
}

// NewPublisher creates a Publisher with the given configuration.
func NewPublisher(cfg Config) *Publisher {
	if len(cfg.Goals) == 0 {
		cfg.Goals = []string{"deploy"}
	}
	return &Publisher{cfg: cfg}
}

// Deploy runs the Maven deploy lifecycle in the project directory.
func (p *Publisher) Deploy(ctx context.Context, projectDir string) error {
	args := append([]string{}, p.cfg.Goals...)
	if p.cfg.SettingsXML != "" {
		args = append(args, "--settings", p.cfg.SettingsXML)
	}
	if p.cfg.SkipTests {
		args = append(args, "-DskipTests")
	}
	for _, profile := range p.cfg.Profiles {
		args = append(args, "-P", profile)
	}
	args = append(args, "--batch-mode")

	cmd := exec.CommandContext(ctx, "mvn", args...)
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("maven: deploy: %w\n%s", err, out)
	}
	return nil
}

// IsMavenAvailable reports whether mvn is installed.
func IsMavenAvailable() bool {
	_, err := exec.LookPath("mvn")
	return err == nil
}

// ReadPOM parses a pom.xml file and returns the minimal project metadata.
func ReadPOM(path string) (*POM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("maven: read pom.xml: %w", err)
	}
	var pom POM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("maven: parse pom.xml: %w", err)
	}
	return &pom, nil
}

// SetVersionWithWrapper runs 'mvn versions:set' to update all version references
// across a multi-module project.
func (p *Publisher) SetVersionWithWrapper(ctx context.Context, projectDir, version string) error {
	args := []string{
		"versions:set",
		fmt.Sprintf("-DnewVersion=%s", version),
		"-DgenerateBackupPoms=false",
		"--batch-mode",
	}
	if p.cfg.SettingsXML != "" {
		args = append(args, "--settings", p.cfg.SettingsXML)
	}

	cmd := exec.CommandContext(ctx, "mvn", args...)
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("maven: versions:set: %w\n%s", err, out)
	}
	return nil
}
