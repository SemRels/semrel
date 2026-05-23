// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package semver provides semantic version parsing and bumping.
// See: https://github.com/GoSemantics/semrel/issues/2
package semver

// Version represents a semantic version.
type Version struct {
Major      int
Minor      int
Patch      int
Prerelease string
Metadata   string
}

// Calculator determines the next semantic version.
// Issue: https://github.com/GoSemantics/semrel/issues/2
type Calculator struct {
// Version bump rules
}

// NewCalculator creates a new SemVer calculator.
func NewCalculator() *Calculator {
return &Calculator{}
}

// NextVersion determines the next version based on commit types.
// Implements semantic versioning bump rules:
// - BREAKING CHANGE or type! → major
// - feat → minor
// - fix → patch
func (c *Calculator) NextVersion(current *Version, hasFeat, hasFix, hasBreaking bool) *Version {
// TODO: Implement version bumping
panic("not implemented")
}
