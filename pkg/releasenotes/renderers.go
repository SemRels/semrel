// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package releasenotes — additional renderers.
package releasenotes

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// artifacthubKindMap maps entry categories to ArtifactHub change kinds.
// See: https://artifacthub.io/docs/topics/annotations/helm/
var artifacthubKindMap = map[string]string{
	"feat":     "added",
	"fix":      "fixed",
	"perf":     "changed",
	"revert":   "removed",
	"docs":     "changed",
	"chore":    "changed",
	"refactor": "changed",
	"security": "security",
}

// artifacthubKind returns the ArtifactHub kind for a given conventional commit type.
func artifacthubKind(commitType string, isBreaking bool) string {
	if isBreaking {
		return "changed"
	}
	if k, ok := artifacthubKindMap[commitType]; ok {
		return k
	}
	return "changed"
}

// RenderArtifactHub renders the release notes as the YAML value of the
// artifacthub.io/changes annotation for use in a Helm Chart.yaml.
//
// Output format:
//
//   - kind: added
//     description: New ingress options
//   - kind: fixed
//     description: Null pointer in init container
//
// See: https://github.com/SemRels/semrel/issues/54
func (r *ReleaseNotes) RenderArtifactHub() string {
	var sb strings.Builder

	writeEntries := func(entries []Entry, overrideKind string) {
		for _, e := range entries {
			kind := overrideKind
			if kind == "" {
				kind = artifacthubKind(e.Type, e.IsBreaking)
			}
			desc := e.Description
			if e.Scope != "" {
				desc = fmt.Sprintf("%s: %s", e.Scope, e.Description)
			}
			// Escape YAML special characters in description
			if strings.ContainsAny(desc, ":#{}[]|>&!") || strings.Contains(desc, "\"") {
				desc = fmt.Sprintf("%q", desc)
			}
			sb.WriteString(fmt.Sprintf("- kind: %s\n  description: %s\n", kind, desc))
		}
	}

	writeEntries(r.Breaking, "security")
	for _, e := range r.Features {
		kind := "added"
		desc := e.Description
		if e.Scope != "" {
			desc = fmt.Sprintf("%s: %s", e.Scope, e.Description)
		}
		if strings.ContainsAny(desc, ":#{}[]|>&!") || strings.Contains(desc, "\"") {
			desc = fmt.Sprintf("%q", desc)
		}
		sb.WriteString(fmt.Sprintf("- kind: %s\n  description: %s\n", kind, desc))
	}
	for _, e := range r.Fixes {
		kind := "fixed"
		desc := e.Description
		if e.Scope != "" {
			desc = fmt.Sprintf("%s: %s", e.Scope, e.Description)
		}
		if strings.ContainsAny(desc, ":#{}[]|>&!") || strings.Contains(desc, "\"") {
			desc = fmt.Sprintf("%q", desc)
		}
		sb.WriteString(fmt.Sprintf("- kind: %s\n  description: %s\n", kind, desc))
	}
	for _, e := range r.Others {
		kind := artifacthubKind(e.Type, false)
		desc := e.Description
		if e.Scope != "" {
			desc = fmt.Sprintf("%s: %s", e.Scope, e.Description)
		}
		if strings.ContainsAny(desc, ":#{}[]|>&!") || strings.Contains(desc, "\"") {
			desc = fmt.Sprintf("%q", desc)
		}
		sb.WriteString(fmt.Sprintf("- kind: %s\n  description: %s\n", kind, desc))
	}

	return sb.String()
}

// ociQuote wraps a value in double-quotes if it contains characters that are
// unsafe in an OCI label value (leading/trailing whitespace, newlines, etc.).
func ociQuote(s string) string {
	if strings.ContainsAny(s, "\n\r\t") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// ociCommitType maps conventional commit types to OCI annotation change types.
var ociCommitType = map[string]string{
	"feat":     "added",
	"fix":      "fixed",
	"perf":     "changed",
	"revert":   "removed",
	"docs":     "changed",
	"chore":    "changed",
	"refactor": "changed",
	"style":    "changed",
	"test":     "changed",
	"build":    "changed",
	"ci":       "changed",
}

// RenderOCI renders the release notes as a set of OCI image annotation labels
// following the opencontainers image-spec label schema.
//
// The primary output is a newline-separated list of key=value pairs
// suitable for use with `docker build --label` or in Dockerfile LABEL instructions:
//
//	org.opencontainers.image.version=v1.2.3
//	org.opencontainers.image.created=2026-05-25T00:00:00Z
//	org.opencontainers.image.revision=
//	org.opencontainers.image.description=<changelog summary>
//
// See: https://github.com/opencontainers/image-spec/blob/main/annotations.md
// See: https://github.com/SemRels/semrel/issues/55
func (r *ReleaseNotes) RenderOCI(revision string) string {
	var sb strings.Builder

	created := r.Date
	if created.IsZero() {
		created = time.Now().UTC()
	}

	// Build a short description: list of change kinds
	var changes []string
	for _, e := range r.Breaking {
		desc := e.Description
		if e.Scope != "" {
			desc = e.Scope + ": " + desc
		}
		changes = append(changes, "BREAKING: "+desc)
	}
	for _, e := range r.Features {
		changes = append(changes, "feat: "+e.Description)
	}
	for _, e := range r.Fixes {
		changes = append(changes, "fix: "+e.Description)
	}
	description := strings.Join(changes, "; ")
	if len(description) > 256 {
		description = description[:253] + "..."
	}

	// Standard OCI annotations
	sb.WriteString(fmt.Sprintf("org.opencontainers.image.version=%s\n", ociQuote(r.Version)))
	sb.WriteString(fmt.Sprintf("org.opencontainers.image.created=%s\n",
		created.UTC().Format("2006-01-02T15:04:05Z")))
	if revision != "" {
		sb.WriteString(fmt.Sprintf("org.opencontainers.image.revision=%s\n", ociQuote(revision)))
	}
	if description != "" {
		sb.WriteString(fmt.Sprintf("org.opencontainers.image.description=%s\n", ociQuote(description)))
	}

	// Extended change-log annotations (one per entry)
	allEntries := make([]struct {
		kind string
		desc string
	}, 0, len(r.Breaking)+len(r.Features)+len(r.Fixes)+len(r.Others))

	for _, e := range r.Breaking {
		desc := e.Description
		if e.Scope != "" {
			desc = e.Scope + ": " + desc
		}
		allEntries = append(allEntries, struct {
			kind string
			desc string
		}{"security", desc})
	}
	for _, e := range r.Features {
		desc := e.Description
		if e.Scope != "" {
			desc = e.Scope + ": " + desc
		}
		allEntries = append(allEntries, struct {
			kind string
			desc string
		}{"added", desc})
	}
	for _, e := range r.Fixes {
		desc := e.Description
		if e.Scope != "" {
			desc = e.Scope + ": " + desc
		}
		allEntries = append(allEntries, struct {
			kind string
			desc string
		}{"fixed", desc})
	}
	for _, e := range r.Others {
		kind := "changed"
		if k, ok := ociCommitType[e.Type]; ok {
			kind = k
		}
		desc := e.Description
		if e.Scope != "" {
			desc = e.Scope + ": " + desc
		}
		allEntries = append(allEntries, struct {
			kind string
			desc string
		}{kind, desc})
	}

	for i, entry := range allEntries {
		sb.WriteString(fmt.Sprintf("org.opencontainers.image.changelog.%d.kind=%s\n", i, entry.kind))
		sb.WriteString(fmt.Sprintf("org.opencontainers.image.changelog.%d.description=%s\n", i, ociQuote(entry.desc)))
	}

	return sb.String()
}

// TemplateData holds structured release data for custom changelog templates.
// See: https://github.com/SemRels/semrel/issues/60
type TemplateData struct {
	Version  string
	Date     string // ISO 8601 date (2006-01-02)
	Breaking []TemplateEntry
	Features []TemplateEntry
	Fixes    []TemplateEntry
	Others   []TemplateEntry
}

// TemplateEntry is the per-commit entry available in templates.
type TemplateEntry struct {
	Type        string
	Scope       string
	Description string
	IsBreaking  bool
	SHA         string
}

// RenderTemplate renders the release notes using a custom Go text/template string.
// The template receives a TemplateData value as its data context.
//
// Built-in template functions:
//   - upper STRING — converts to uppercase
//   - lower STRING — converts to lowercase
//   - truncate N STRING — truncates string to N characters
//
// Example template:
//
//	## {{.Version}} ({{.Date}})
//	{{range .Features}}- feat({{.Scope}}): {{.Description}}
//	{{end}}
//
// See: https://github.com/SemRels/semrel/issues/60
func (r *ReleaseNotes) RenderTemplate(tmplSrc string) (string, error) {
	date := r.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	toEntry := func(e Entry) TemplateEntry {
		return TemplateEntry{
			Type:        e.Type,
			Scope:       e.Scope,
			Description: e.Description,
			IsBreaking:  e.IsBreaking,
			SHA:         e.SHA,
		}
	}

	data := TemplateData{
		Version: r.Version,
		Date:    date.Format("2006-01-02"),
	}
	for _, e := range r.Breaking {
		data.Breaking = append(data.Breaking, toEntry(e))
	}
	for _, e := range r.Features {
		data.Features = append(data.Features, toEntry(e))
	}
	for _, e := range r.Fixes {
		data.Fixes = append(data.Fixes, toEntry(e))
	}
	for _, e := range r.Others {
		data.Others = append(data.Others, toEntry(e))
	}

	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"truncate": func(n int, s string) string {
			r := []rune(s)
			if len(r) > n {
				return string(r[:n]) + "…"
			}
			return s
		},
	}

	tmpl, err := template.New("changelog").Funcs(funcMap).Parse(tmplSrc)
	if err != nil {
		return "", fmt.Errorf("parsing changelog template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing changelog template: %w", err)
	}

	return buf.String(), nil
}
