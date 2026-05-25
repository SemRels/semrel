// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Additional changelog renderers: NuGet, PyPI/CHANGES.rst, Maven/Gradle.
//
// See:
//   - https://github.com/SemRels/semrel/issues/56 (NuGet .csproj releaseNotes)
//   - https://github.com/SemRels/semrel/issues/57 (PyPI CHANGES.rst / CHANGES.md)
//   - https://github.com/SemRels/semrel/issues/58 (Maven POM description embed)
package releasenotes

import (
	"fmt"
	"strings"
)

// RenderNuGetReleaseNotes returns the release notes formatted for embedding
// in a .csproj <PackageReleaseNotes> or .nuspec <releaseNotes> element.
//
// The output is plain text suitable for XML text content (no CDATA wrapper).
// Callers embedding into XML should escape as needed; this function emits a
// compact human-readable format that displays well in NuGet clients.
//
// Example output:
//
//	v1.2.0
//
//	Breaking Changes:
//	  - (api) remove legacy endpoint
//
//	Features:
//	  - (auth) support OAuth2 PKCE
//
//	Bug Fixes:
//	  - prevent nil dereference on empty input
//
// See: https://github.com/SemRels/semrel/issues/56
func (r *ReleaseNotes) RenderNuGetReleaseNotes(version string) string {
	var sb strings.Builder

	if version != "" {
		sb.WriteString(version)
		sb.WriteString("\n\n")
	}

	if len(r.Breaking) > 0 {
		sb.WriteString("Breaking Changes:\n")
		for _, e := range r.Breaking {
			sb.WriteString("  - ")
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("(%s) ", e.Scope))
			}
			sb.WriteString(e.Description)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(r.Features) > 0 {
		sb.WriteString("Features:\n")
		for _, e := range r.Features {
			sb.WriteString("  - ")
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("(%s) ", e.Scope))
			}
			sb.WriteString(e.Description)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(r.Fixes) > 0 {
		sb.WriteString("Bug Fixes:\n")
		for _, e := range r.Fixes {
			sb.WriteString("  - ")
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("(%s) ", e.Scope))
			}
			sb.WriteString(e.Description)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// RenderPyPIChangelog returns the release notes formatted as a reStructuredText
// (RST) changelog entry suitable for CHANGES.rst or as a GitHub release body
// for Python packages uploaded to PyPI.
//
// Example output:
//
//	v1.2.0 (2026-01-15)
//	-------------------
//
//	**Breaking Changes**
//
//	- ``api``: remove legacy endpoint
//
//	**New Features**
//
//	- ``auth``: support OAuth2 PKCE
//
//	**Bug Fixes**
//
//	- prevent nil dereference on empty input
//
// See: https://github.com/SemRels/semrel/issues/57
func (r *ReleaseNotes) RenderPyPIChangelog(version, date string) string {
	var sb strings.Builder

	heading := version
	if date != "" {
		heading = fmt.Sprintf("%s (%s)", version, date)
	}
	sb.WriteString(heading)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", len(heading)))
	sb.WriteString("\n")

	if len(r.Breaking) > 0 {
		sb.WriteString("\n**Breaking Changes**\n\n")
		for _, e := range r.Breaking {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- ``%s``: %s\n", e.Scope, e.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", e.Description))
			}
		}
	}

	if len(r.Features) > 0 {
		sb.WriteString("\n**New Features**\n\n")
		for _, e := range r.Features {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- ``%s``: %s\n", e.Scope, e.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", e.Description))
			}
		}
	}

	if len(r.Fixes) > 0 {
		sb.WriteString("\n**Bug Fixes**\n\n")
		for _, e := range r.Fixes {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- ``%s``: %s\n", e.Scope, e.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", e.Description))
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// RenderMavenChangelog returns the release notes formatted as a Maven/Gradle
// changelog section suitable for embedding in a POM <description>, a
// CHANGES.md, or a GitHub Packages release body.
//
// The output is GitHub-flavored Markdown, which renders well in Maven Central
// and GitHub Packages release pages.
//
// Example output:
//
//	## v1.2.0
//
//	### ⚠ Breaking Changes
//
//	- **api:** remove legacy endpoint
//
//	### ✨ Features
//
//	- **auth:** support OAuth2 PKCE
//
//	### 🐛 Bug Fixes
//
//	- prevent nil dereference on empty input
//
// See: https://github.com/SemRels/semrel/issues/58
func (r *ReleaseNotes) RenderMavenChangelog(version string) string {
	var sb strings.Builder

	if version != "" {
		sb.WriteString(fmt.Sprintf("## %s\n", version))
	}

	if len(r.Breaking) > 0 {
		sb.WriteString("\n### ⚠ Breaking Changes\n\n")
		for _, e := range r.Breaking {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- **%s:** %s\n", e.Scope, e.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", e.Description))
			}
		}
	}

	if len(r.Features) > 0 {
		sb.WriteString("\n### ✨ Features\n\n")
		for _, e := range r.Features {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- **%s:** %s\n", e.Scope, e.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", e.Description))
			}
		}
	}

	if len(r.Fixes) > 0 {
		sb.WriteString("\n### 🐛 Bug Fixes\n\n")
		for _, e := range r.Fixes {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- **%s:** %s\n", e.Scope, e.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", e.Description))
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
