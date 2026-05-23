// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package changelog provides changelog generation.
// See: https://github.com/GoSemantics/semrel/issues/11
package changelog

// Generator generates changelog from commits.
type Generator struct {
	_ struct{} // Placeholder for future fields
}

// NewGenerator creates a new changelog generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates a changelog entry for a version.
// Issue: https://github.com/GoSemantics/semrel/issues/11
// Formats commits as markdown grouped by type (Features, Fixes, Breaking Changes).
func (g *Generator) Generate(version string, commits []interface{}) string {
	// TODO: Implement changelog generation
	panic("not implemented")
}
