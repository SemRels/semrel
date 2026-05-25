// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package changelog provides changelog generation.
// See: https://github.com/SemRels/semrel/issues/11
package changelog

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoSemantics/semrel/pkg/commits"
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
	var breaking, features, fixes, others []string

	for _, c := range cs {
		if c == nil {
			continue
		}
		line := formatCommitLine(c)
		switch {
		case c.IsBreakingChange:
			breaking = append(breaking, line)
		case c.Type == "feat":
			features = append(features, line)
		case c.Type == "fix" || c.Type == "perf" || c.Type == "revert":
			fixes = append(fixes, line)
		case c.Type != "":
			others = append(others, line)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%s)\n\n", version, time.Now().UTC().Format("2006-01-02")))

	if len(breaking) > 0 {
		sb.WriteString("### ⚠ BREAKING CHANGES\n\n")
		for _, l := range breaking {
			sb.WriteString("* " + l + "\n")
		}
		sb.WriteString("\n")
	}
	if len(features) > 0 {
		sb.WriteString("### Features\n\n")
		for _, l := range features {
			sb.WriteString("* " + l + "\n")
		}
		sb.WriteString("\n")
	}
	if len(fixes) > 0 {
		sb.WriteString("### Bug Fixes\n\n")
		for _, l := range fixes {
			sb.WriteString("* " + l + "\n")
		}
		sb.WriteString("\n")
	}
	if len(others) > 0 {
		sb.WriteString("### Other Changes\n\n")
		for _, l := range others {
			sb.WriteString("* " + l + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatCommitLine(c *commits.Commit) string {
	if c.Scope != "" {
		return fmt.Sprintf("**%s:** %s", c.Scope, c.Description)
	}
	return c.Description
}
