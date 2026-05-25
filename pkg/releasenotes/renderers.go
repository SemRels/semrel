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
	"feat":    "added",
	"fix":     "fixed",
	"perf":    "changed",
	"revert":  "removed",
	"docs":    "changed",
	"chore":   "changed",
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
//	- kind: added
//	  description: New ingress options
//	- kind: fixed
//	  description: Null pointer in init container
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

// TemplateData is the context passed to custom Go text/template changelog templates.
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
