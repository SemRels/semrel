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
