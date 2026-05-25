// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package semver provides semantic version parsing and bumping.
// See: https://github.com/SemRels/semrel/issues/2
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Metadata   string
}

// String returns the version as a semver string (without leading "v").
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Metadata != "" {
		s += "+" + v.Metadata
	}
	return s
}

// ParseVersion parses a version string, optionally prefixed with "v".
// Returns a 0.0.0 version if the string is empty or cannot be parsed.
func ParseVersion(s string) (*Version, error) {
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return &Version{}, nil
	}

	// Strip metadata
	metadata := ""
	if idx := strings.Index(s, "+"); idx >= 0 {
		metadata = s[idx+1:]
		s = s[:idx]
	}

	// Strip prerelease
	prerelease := ""
	if idx := strings.Index(s, "-"); idx >= 0 {
		prerelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid semver %q: expected MAJOR.MINOR.PATCH", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version %q: %w", parts[2], err)
	}

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Metadata:   metadata,
	}, nil
}

// Calculator determines the next semantic version.
// Issue: https://github.com/SemRels/semrel/issues/2
type Calculator struct{}

// NewCalculator creates a new SemVer calculator.
func NewCalculator() *Calculator {
	return &Calculator{}
}

// NextVersion determines the next version based on commit analysis results.
// Breaking changes bump major (resetting minor+patch).
// feat bumps minor (resetting patch).
// fix/perf/revert bump patch.
// Returns nil if no releasable commits were found.
func (c *Calculator) NextVersion(current *Version, hasFeat, hasFix, hasBreaking bool) *Version {
	if !hasFeat && !hasFix && !hasBreaking {
		return nil
	}

	next := &Version{
		Major: current.Major,
		Minor: current.Minor,
		Patch: current.Patch,
	}

	switch {
	case hasBreaking:
		next.Major++
		next.Minor = 0
		next.Patch = 0
	case hasFeat:
		next.Minor++
		next.Patch = 0
	default: // hasFix
		next.Patch++
	}

	return next
}

// NextVersionForBranch is like NextVersion but respects maintenance branch restrictions.
// On a maintenance branch only patch bumps are allowed; major/minor bumps are capped to patch.
// See: https://github.com/SemRels/semrel/issues/95
func (c *Calculator) NextVersionForBranch(current *Version, hasFeat, hasFix, hasBreaking, isMaintenance bool) *Version {
	if isMaintenance {
		// On maintenance branches only bug/security patches are allowed.
		if !hasFix && !hasBreaking && !hasFeat {
			return nil
		}
		next := &Version{
			Major: current.Major,
			Minor: current.Minor,
			Patch: current.Patch + 1,
		}
		return next
	}
	return c.NextVersion(current, hasFeat, hasFix, hasBreaking)
}

// NextPrereleaseVersion computes the next pre-release version for a given channel.
//
// Algorithm:
//  1. If current is not a pre-release: compute next stable base, start at <base>-<channel>.1.
//  2. If current is already a pre-release for the same channel:
//     - If hasBreaking and major must go higher: start fresh at new major base.
//     - Otherwise: increment the pre-release counter (base doesn't change).
//  3. If current is a pre-release for a different channel: start at <current base>-<channel>.1.
//
// Example: channel="beta", current=1.2.3, hasFeat=true    → 1.3.0-beta.1
//
//	channel="beta", current=1.3.0-beta.1, hasFeat=true → 1.3.0-beta.2
//	channel="beta", current=1.3.0-beta.2, hasBreaking=true → 2.0.0-beta.1
//	channel="beta", current=1.3.0-alpha.5, hasFeat=true → 1.3.0-beta.1
//
// See: https://github.com/SemRels/semrel/issues/45
func (c *Calculator) NextPrereleaseVersion(current *Version, hasFeat, hasFix, hasBreaking bool, channel string) *Version {
	if !hasFeat && !hasFix && !hasBreaking {
		return nil
	}
	if channel == "" {
		return c.NextVersion(current, hasFeat, hasFix, hasBreaking)
	}

	currentBase := &Version{Major: current.Major, Minor: current.Minor, Patch: current.Patch}

	if current.Prerelease != "" {
		counter := prereleaseCounter(current.Prerelease, channel)

		if counter > 0 {
			// Same channel — check if a major bump is required
			if hasBreaking {
				// Breaking change requires a new major base
				newBase := &Version{Major: currentBase.Major + 1, Minor: 0, Patch: 0}
				newBase.Prerelease = channel + ".1"
				return newBase
			}
			// Stay on the same base, increment counter
			result := &Version{Major: currentBase.Major, Minor: currentBase.Minor, Patch: currentBase.Patch}
			result.Prerelease = fmt.Sprintf("%s.%d", channel, counter+1)
			return result
		}

		// Different channel — start fresh series on the SAME base
		result := &Version{Major: currentBase.Major, Minor: currentBase.Minor, Patch: currentBase.Patch}
		result.Prerelease = channel + ".1"
		return result
	}

	// Current is stable — compute next stable base and start fresh
	base := c.NextVersion(currentBase, hasFeat, hasFix, hasBreaking)
	if base == nil {
		return nil
	}
	base.Prerelease = channel + ".1"
	return base
}

// prereleaseCounter returns the numeric counter from a pre-release string like "beta.3".
// Returns 0 if the string does not match the expected format for the given channel.
func prereleaseCounter(prerelease, channel string) int {
	prefix := channel + "."
	if !strings.HasPrefix(prerelease, prefix) {
		return 0
	}
	n, err := strconv.Atoi(prerelease[len(prefix):])
	if err != nil || n < 1 {
		return 0
	}
	return n
}

// ForcePatch always bumps the patch component, regardless of commits.
// Useful with --force-bump-patch-version to guarantee a release even when
// no releasable conventional commits were found.
// See: https://github.com/SemRels/semrel/issues/96
func (c *Calculator) ForcePatch(current *Version) *Version {
	return &Version{
		Major: current.Major,
		Minor: current.Minor,
		Patch: current.Patch + 1,
	}
}

// BumpFromRules analyses commits against release rules and returns the bump level.
func BumpFromRules(commitTypes []string, rules map[string]string, hasBreaking bool) string {
	if hasBreaking {
		return "major"
	}
	best := ""
	order := map[string]int{"major": 3, "minor": 2, "patch": 1}
	for _, t := range commitTypes {
		if bump, ok := rules[t]; ok {
			if order[bump] > order[best] {
				best = bump
			}
		}
	}
	return best
}
