// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package changelog provides changelog generation.
// See: https://github.com/SemRels/semrel/issues/11
package changelog

import (
	"fmt"
	"os"
	"strings"

	"github.com/SemRels/semrel/pkg/commits"
	"github.com/SemRels/semrel/pkg/releasenotes"
)

// Generator generates changelog from commits.
type Generator struct{}

// NewGenerator creates a new changelog generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates a Markdown changelog entry for the given version and commits.
// Commits are grouped by type: Breaking Changes, Features, Bug Fixes, then Others.
// Issue: https://github.com/SemRels/semrel/issues/11
func (g *Generator) Generate(version string, cs []*commits.Commit) string {
	rn := releasenotes.Build(version, cs)
	return rn.RenderMarkdown()
}

// WriteFile writes a changelog entry to a new file (overwrites if exists).
func (g *Generator) WriteFile(path, version string, cs []*commits.Commit) error {
	content := g.Generate(version, cs)
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("changelog: write file %q: %w", path, err)
	}
	return nil
}

// PrependToFile prepends a new changelog entry to an existing CHANGELOG.md file.
// If the file does not exist, it is created.
func (g *Generator) PrependToFile(path, version string, cs []*commits.Commit) error {
	newEntry := g.Generate(version, cs)

	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	// Remove a leading "# Changelog" header from existing content to avoid duplication
	existing = strings.TrimPrefix(existing, "# Changelog\n\n")
	existing = strings.TrimPrefix(existing, "# Changelog\n")

	var sb strings.Builder
	sb.WriteString("# Changelog\n\n")
	sb.WriteString(strings.TrimSpace(newEntry))
	sb.WriteString("\n")
	if existing != "" {
		sb.WriteString("\n")
		sb.WriteString(strings.TrimSpace(existing))
		sb.WriteString("\n")
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("changelog: prepend to file %q: %w", path, err)
	}
	return nil
}
