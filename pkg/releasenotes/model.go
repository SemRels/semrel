// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package releasenotes defines the structured ReleaseNotes model used across
// all changelog renderers and output formats.
// See: https://github.com/SemRels/semrel/issues/52
package releasenotes

import (
	"fmt"
	"strings"
	"time"
)

// Entry represents a single change entry in the release notes.
type Entry struct {
	Type        string // Conventional commit type (feat, fix, chore, …)
	Scope       string // Optional scope
	Description string // Commit description text
	IsBreaking  bool   // True if this is a breaking change
	SHA         string // Optional commit SHA (abbreviated)
}

// ReleaseNotes is the structured representation of changes in a single release.
// It is the canonical intermediate form: produce it once, render it many ways.
// See: https://github.com/SemRels/semrel/issues/52
type ReleaseNotes struct {
	// Version is the release tag (e.g. "v1.2.0").
	Version string
	// Date is the release date. If zero, defaults to today (UTC) when rendering.
	Date time.Time
	// Breaking contains breaking-change entries.
	Breaking []Entry
	// Features contains feat-type entries that are not breaking.
	Features []Entry
	// Fixes contains fix/perf/revert entries.
	Fixes []Entry
	// Others contains all remaining typed entries.
	Others []Entry
}

// IsEmpty returns true when there are no entries at all.
func (r ReleaseNotes) IsEmpty() bool {
	return len(r.Breaking)+len(r.Features)+len(r.Fixes)+len(r.Others) == 0
}

// HasBreaking returns true when there is at least one breaking change.
func (r ReleaseNotes) HasBreaking() bool { return len(r.Breaking) > 0 }

// BumpLevel returns the semver bump level implied by the release notes:
// "major", "minor", "patch", or "" (no release).
func (r ReleaseNotes) BumpLevel() string {
	if r.IsEmpty() {
		return ""
	}
	if r.HasBreaking() {
		return "major"
	}
	if len(r.Features) > 0 {
		return "minor"
	}
	return "patch"
}

// RenderMarkdown renders the release notes as a CHANGELOG.md-style Markdown
// fragment (Keep-a-Changelog inspired).
func (r *ReleaseNotes) RenderMarkdown() string {
	date := r.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%s)\n\n", r.Version, date.Format("2006-01-02")))

	writeSection := func(heading string, entries []Entry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString("### " + heading + "\n\n")
		for _, e := range entries {
			sb.WriteString("* " + formatEntry(e) + "\n")
		}
		sb.WriteString("\n")
	}

	writeSection("⚠ BREAKING CHANGES", r.Breaking)
	writeSection("Features", r.Features)
	writeSection("Bug Fixes", r.Fixes)
	writeSection("Other Changes", r.Others)

	return sb.String()
}

// SectionConfig defines the mapping from commit categories to Keep-a-Changelog section names.
// See: https://keepachangelog.com/en/1.0.0/
type SectionConfig struct {
	Breaking string // Default: "⚠ Breaking Changes"
	Features string // Default: "Added"
	Fixes    string // Default: "Fixed"
	Others   string // Default: "Changed"
}

// DefaultSectionConfig returns the standard Keep-a-Changelog 1.0.0 section names.
func DefaultSectionConfig() SectionConfig {
	return SectionConfig{
		Breaking: "⚠ Breaking Changes",
		Features: "Added",
		Fixes:    "Fixed",
		Others:   "Changed",
	}
}

// RenderKeepAChangelog renders the release notes using the Keep-a-Changelog 1.0.0 format.
// Version is rendered as [1.2.0] with an ISO date: ## [1.2.0] - 2026-05-22
// Section names follow the Keep-a-Changelog spec (configurable via cfg).
// See: https://github.com/SemRels/semrel/issues/53
func (r *ReleaseNotes) RenderKeepAChangelog(cfg SectionConfig) string {
	date := r.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	// Strip leading "v" prefix for the bracketed version (KaC convention)
	ver := strings.TrimPrefix(r.Version, "v")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## [%s] - %s\n\n", ver, date.Format("2006-01-02")))

	writeSection := func(heading string, entries []Entry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString("### " + heading + "\n\n")
		for _, e := range entries {
			sb.WriteString("- " + formatEntry(e) + "\n")
		}
		sb.WriteString("\n")
	}

	writeSection(cfg.Breaking, r.Breaking)
	writeSection(cfg.Features, r.Features)
	writeSection(cfg.Fixes, r.Fixes)
	writeSection(cfg.Others, r.Others)

	return sb.String()
}

// RenderText renders a compact plain-text summary (useful for notifications).
func (r *ReleaseNotes) RenderText() string {
	date := r.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%s)\n", r.Version, date.Format("2006-01-02")))

	if r.HasBreaking() {
		sb.WriteString(fmt.Sprintf("  ⚠ %d breaking change(s)\n", len(r.Breaking)))
	}
	if len(r.Features) > 0 {
		sb.WriteString(fmt.Sprintf("  ✨ %d new feature(s)\n", len(r.Features)))
	}
	if len(r.Fixes) > 0 {
		sb.WriteString(fmt.Sprintf("  🐛 %d bug fix(es)\n", len(r.Fixes)))
	}
	if len(r.Others) > 0 {
		sb.WriteString(fmt.Sprintf("  🔧 %d other change(s)\n", len(r.Others)))
	}
	return sb.String()
}

func formatEntry(e Entry) string {
	if e.Scope != "" {
		return fmt.Sprintf("**%s:** %s", e.Scope, e.Description)
	}
	return e.Description
}
