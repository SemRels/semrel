// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package changelog provides changelog generation.
// See: https://github.com/SemRels/semrel/issues/11
package changelog

import (
	"github.com/GoSemantics/semrel/pkg/commits"
	"github.com/GoSemantics/semrel/pkg/releasenotes"
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
