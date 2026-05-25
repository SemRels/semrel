// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package sbom generates Software Bill of Materials documents in CycloneDX
// and SPDX formats for a release.
//
// A SBOM documents all software components, licenses, and relationships
// so that consumers can understand what is included in a release. This is
// increasingly required by security frameworks (SLSA, OpenSSF, NTIA).
//
// Supported formats:
//   - CycloneDX 1.4 (JSON)
//   - SPDX 2.3 (tag-value text)
//
// See: https://github.com/SemRels/semrel/issues/34
package sbom

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Format specifies the SBOM output format.
type Format string

const (
	// FormatCycloneDX produces a CycloneDX 1.4 JSON document.
	FormatCycloneDX Format = "cyclonedx"
	// FormatSPDX produces a SPDX 2.3 tag-value text document.
	FormatSPDX Format = "spdx"
)

// Component represents a software component to include in the SBOM.
type Component struct {
	// Name is the component name (e.g. "github.com/foo/bar").
	Name string
	// Version is the component version (e.g. "v1.2.3").
	Version string
	// Type is the component type (e.g. "library", "application").
	Type string
	// License is the SPDX license expression (e.g. "Apache-2.0").
	License string
	// PURL is the Package URL (e.g. "pkg:golang/github.com/foo/bar@v1.2.3").
	PURL string
}

// Document is the generated SBOM.
type Document struct {
	// Format is the SBOM format.
	Format Format
	// Content is the serialized document.
	Content string
}

// Metadata holds top-level SBOM metadata for the subject release.
type Metadata struct {
	// Name is the software name (e.g. "semrel").
	Name string
	// Version is the software version being released (e.g. "1.2.3").
	Version string
	// Supplier is the organization or person producing the SBOM.
	Supplier string
	// Timestamp overrides the generation time (zero = use now).
	Timestamp time.Time
}

// Generator creates SBOMs for a release.
type Generator struct {
	meta       Metadata
	components []Component
}

// NewGenerator creates a generator for the given release metadata.
func NewGenerator(meta Metadata) *Generator {
	return &Generator{meta: meta}
}

// AddComponent adds a component to the SBOM.
func (g *Generator) AddComponent(c Component) {
	if c.Type == "" {
		c.Type = "library"
	}
	g.components = append(g.components, c)
}

// Generate produces the SBOM document in the requested format.
func (g *Generator) Generate(format Format) (*Document, error) {
	switch format {
	case FormatCycloneDX:
		content, err := g.generateCycloneDX()
		if err != nil {
			return nil, err
		}
		return &Document{Format: FormatCycloneDX, Content: content}, nil
	case FormatSPDX:
		content := g.generateSPDX()
		return &Document{Format: FormatSPDX, Content: content}, nil
	default:
		return nil, fmt.Errorf("unsupported SBOM format: %q", format)
	}
}

// ---- CycloneDX 1.4 --------------------------------------------------------

type cycloneDXDoc struct {
	BOMFormat    string         `json:"bomFormat"`
	SpecVersion  string         `json:"specVersion"`
	SerialNumber string         `json:"serialNumber"`
	Version      int            `json:"version"`
	Metadata     cdxMetadata    `json:"metadata"`
	Components   []cdxComponent `json:"components"`
}

type cdxMetadata struct {
	Timestamp string       `json:"timestamp"`
	Component cdxComponent `json:"component"`
}

type cdxComponent struct {
	Type     string       `json:"type"`
	Name     string       `json:"name"`
	Version  string       `json:"version,omitempty"`
	PURL     string       `json:"purl,omitempty"`
	Licenses []cdxLicense `json:"licenses,omitempty"`
}

type cdxLicense struct {
	License cdxLicenseID `json:"license"`
}

type cdxLicenseID struct {
	ID string `json:"id"`
}

func (g *Generator) generateCycloneDX() (string, error) {
	ts := g.meta.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	doc := cycloneDXDoc{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.4",
		SerialNumber: fmt.Sprintf("urn:uuid:semrel-%s-%d", g.meta.Name, ts.Unix()),
		Version:      1,
		Metadata: cdxMetadata{
			Timestamp: ts.Format(time.RFC3339),
			Component: cdxComponent{
				Type:    "application",
				Name:    g.meta.Name,
				Version: g.meta.Version,
			},
		},
	}

	for _, c := range g.components {
		comp := cdxComponent{
			Type:    c.Type,
			Name:    c.Name,
			Version: c.Version,
			PURL:    c.PURL,
		}
		if c.License != "" {
			comp.Licenses = []cdxLicense{{License: cdxLicenseID{ID: c.License}}}
		}
		doc.Components = append(doc.Components, comp)
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ---- SPDX 2.3 tag-value ---------------------------------------------------

func (g *Generator) generateSPDX() string {
	ts := g.meta.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	var sb strings.Builder

	sb.WriteString("SPDXVersion: SPDX-2.3\n")
	sb.WriteString("DataLicense: CC0-1.0\n")
	sb.WriteString("SPDXID: SPDXRef-DOCUMENT\n")
	sb.WriteString(fmt.Sprintf("DocumentName: %s-%s\n", g.meta.Name, g.meta.Version))
	sb.WriteString(fmt.Sprintf("DocumentNamespace: https://spdx.org/spdxdocs/%s-%s\n", g.meta.Name, g.meta.Version))
	sb.WriteString("Creator: Tool: semrel\n")
	if g.meta.Supplier != "" {
		sb.WriteString(fmt.Sprintf("Creator: Organization: %s\n", g.meta.Supplier))
	}
	sb.WriteString(fmt.Sprintf("Created: %s\n\n", ts.Format(time.RFC3339)))

	// Subject package
	sb.WriteString("PackageName: " + g.meta.Name + "\n")
	sb.WriteString("SPDXID: SPDXRef-Package\n")
	sb.WriteString("PackageVersion: " + g.meta.Version + "\n")
	sb.WriteString("FilesAnalyzed: false\n\n")

	// Components
	for i, c := range g.components {
		ref := fmt.Sprintf("SPDXRef-Component-%d", i+1)
		sb.WriteString(fmt.Sprintf("PackageName: %s\n", c.Name))
		sb.WriteString(fmt.Sprintf("SPDXID: %s\n", ref))
		if c.Version != "" {
			sb.WriteString(fmt.Sprintf("PackageVersion: %s\n", c.Version))
		}
		if c.PURL != "" {
			sb.WriteString(fmt.Sprintf("ExternalRef: PACKAGE-MANAGER purl %s\n", c.PURL))
		}
		if c.License != "" {
			sb.WriteString(fmt.Sprintf("PackageLicenseConcluded: %s\n", c.License))
			sb.WriteString(fmt.Sprintf("PackageLicenseDeclared: %s\n", c.License))
		} else {
			sb.WriteString("PackageLicenseConcluded: NOASSERTION\n")
			sb.WriteString("PackageLicenseDeclared: NOASSERTION\n")
		}
		sb.WriteString("FilesAnalyzed: false\n\n")
	}

	return sb.String()
}
