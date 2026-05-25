// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package gradle provides a Gradle build tool plugin for updating versions and
// publishing Java/Kotlin/Groovy projects to Maven repositories.
package gradle

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// BuildFile represents a parsed Gradle build file.
type BuildFile struct {
	// Path is the file path (build.gradle or build.gradle.kts).
	Path string
	// Version is the project version found in the file.
	Version string
}

// UpdateVersion reads a Gradle build file (build.gradle or build.gradle.kts),
// updates the version property, and writes it back.
func UpdateVersion(path, version string) (*BuildFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gradle: read build file: %w", err)
	}

	updated, err := replaceVersion(data, version)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return nil, fmt.Errorf("gradle: write build file: %w", err)
	}
	return &BuildFile{Path: path, Version: version}, nil
}

// replaceVersion updates the version assignment in a Gradle build file.
// Supports both Groovy DSL (version = '...') and Kotlin DSL (version = "...").
func replaceVersion(data []byte, version string) ([]byte, error) {
	// Match: version = "x.y.z" or version = 'x.y.z' or version = x.y.z
	groovyRe := regexp.MustCompile(`(?m)^(\s*version\s*=\s*)['"][^'"]*['"]`)
	kotlinRe := regexp.MustCompile(`(?m)^(\s*version\s*=\s*)"[^"]*"`)

	content := string(data)
	var updated string

	if groovyRe.MatchString(content) {
		// Preserve the quote style
		updated = groovyRe.ReplaceAllStringFunc(content, func(match string) string {
			if strings.Contains(match, `"`) {
				return regexp.MustCompile(`"[^"]*"`).ReplaceAllString(match, `"`+version+`"`)
			}
			return regexp.MustCompile(`'[^']*'`).ReplaceAllString(match, `'`+version+`'`)
		})
	} else if kotlinRe.MatchString(content) {
		updated = kotlinRe.ReplaceAllString(content, `${1}"`+version+`"`)
	} else {
		return nil, fmt.Errorf("gradle: version property not found in build file")
	}

	return []byte(updated), nil
}

// ReadVersion reads the version from a Gradle build file.
func ReadVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("gradle: read build file: %w", err)
	}

	re := regexp.MustCompile(`(?m)^\s*version\s*=\s*['"]([^'"]+)['"]`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		return "", fmt.Errorf("gradle: version not found in build file")
	}
	return m[1], nil
}

// Publisher publishes Gradle projects to a Maven repository.
type Publisher struct {
	cfg Config
}

// Config holds Gradle publishing configuration.
type Config struct {
	// PublishTask is the Gradle task to run (defaults to "publish").
	PublishTask string
	// Repository is the Maven repository URL for publishing.
	Repository string
	// Username for repository authentication.
	Username string
	// Password for repository authentication.
	Password string
}

// NewPublisher creates a Publisher with the given configuration.
func NewPublisher(cfg Config) *Publisher {
	if cfg.PublishTask == "" {
		cfg.PublishTask = "publish"
	}
	return &Publisher{cfg: cfg}
}

// Publish runs the Gradle publish task in the project directory.
func (p *Publisher) Publish(ctx context.Context, projectDir string) error {
	gradleCmd := "./gradlew"
	if _, err := os.Stat(projectDir + "/gradlew"); os.IsNotExist(err) {
		gradleCmd = "gradle"
	}

	cmd := exec.CommandContext(ctx, gradleCmd, p.cfg.PublishTask, "--no-daemon")
	cmd.Dir = projectDir
	env := os.Environ()
	if p.cfg.Username != "" {
		env = append(env,
			"GRADLE_PUBLISH_USERNAME="+p.cfg.Username,
			"GRADLE_PUBLISH_PASSWORD="+p.cfg.Password,
		)
	}
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gradle: publish task: %w\n%s", err, out)
	}
	return nil
}

// IsGradleAvailable reports whether gradle or gradlew is available.
func IsGradleAvailable() bool {
	if _, err := exec.LookPath("gradle"); err == nil {
		return true
	}
	_, err := os.Stat("gradlew")
	return err == nil
}

// UpdateVersionInGradleProperties updates the version in a gradle.properties file.
func UpdateVersionInGradleProperties(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("gradle: read gradle.properties: %w", err)
	}

	versionRe := regexp.MustCompile(`(?m)^(version\s*=\s*)\S+`)
	var lines []string
	versionSet := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if !versionSet && versionRe.MatchString(strings.TrimSpace(line)) {
			line = versionRe.ReplaceAllString(line, "${1}"+version)
			versionSet = true
		}
		lines = append(lines, line)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
