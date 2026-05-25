// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package sbom_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/GoSemantics/semrel/pkg/sbom"
)

var fixedTime = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

func newMeta() sbom.Metadata {
	return sbom.Metadata{
		Name:      "semrel",
		Version:   "1.2.3",
		Supplier:  "SemRels",
		Timestamp: fixedTime,
	}
}

func TestGenerateCycloneDX_Structure(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	g.AddComponent(sbom.Component{
		Name:    "github.com/foo/bar",
		Version: "v1.0.0",
		License: "MIT",
		PURL:    "pkg:golang/github.com/foo/bar@v1.0.0",
	})

	doc, err := g.Generate(sbom.FormatCycloneDX)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Format != sbom.FormatCycloneDX {
		t.Errorf("expected CycloneDX format, got %q", doc.Format)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(doc.Content), &raw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, doc.Content)
	}
	if raw["bomFormat"] != "CycloneDX" {
		t.Errorf("expected bomFormat CycloneDX, got %v", raw["bomFormat"])
	}
	if raw["specVersion"] != "1.4" {
		t.Errorf("expected specVersion 1.4, got %v", raw["specVersion"])
	}
}

func TestGenerateCycloneDX_Components(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	g.AddComponent(sbom.Component{Name: "lib-a", Version: "v2.0.0", License: "Apache-2.0"})
	g.AddComponent(sbom.Component{Name: "lib-b", Version: "v3.0.0"})

	doc, err := g.Generate(sbom.FormatCycloneDX)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]any
	json.Unmarshal([]byte(doc.Content), &raw)
	components, ok := raw["components"].([]any)
	if !ok || len(components) != 2 {
		t.Fatalf("expected 2 components, got %v", raw["components"])
	}
}

func TestGenerateCycloneDX_DefaultComponentType(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	g.AddComponent(sbom.Component{Name: "my-lib"}) // no Type set

	doc, err := g.Generate(sbom.FormatCycloneDX)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.Content, `"library"`) {
		t.Error("expected default type 'library' in output")
	}
}

func TestGenerateSPDX_Structure(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	g.AddComponent(sbom.Component{
		Name:    "github.com/baz/qux",
		Version: "v0.1.0",
		License: "MIT",
		PURL:    "pkg:golang/github.com/baz/qux@v0.1.0",
	})

	doc, err := g.Generate(sbom.FormatSPDX)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Format != sbom.FormatSPDX {
		t.Errorf("expected SPDX format, got %q", doc.Format)
	}

	content := doc.Content
	requiredFields := []string{
		"SPDXVersion: SPDX-2.3",
		"DataLicense: CC0-1.0",
		"SPDXID: SPDXRef-DOCUMENT",
		"Creator: Tool: semrel",
		"PackageName: semrel",
		"PackageName: github.com/baz/qux",
		"PackageLicenseConcluded: MIT",
		"ExternalRef: PACKAGE-MANAGER purl pkg:golang/github.com/baz/qux@v0.1.0",
	}
	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("SPDX output missing %q", field)
		}
	}
}

func TestGenerateSPDX_NoAssertionLicense(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	g.AddComponent(sbom.Component{Name: "unknown-lib"})

	doc, _ := g.Generate(sbom.FormatSPDX)
	if !strings.Contains(doc.Content, "PackageLicenseConcluded: NOASSERTION") {
		t.Error("expected NOASSERTION for component without license")
	}
}

func TestGenerate_UnknownFormat(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	_, err := g.Generate("invalid-format")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestGenerateCycloneDX_NoComponents(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	doc, err := g.Generate(sbom.FormatCycloneDX)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.Content, `"CycloneDX"`) {
		t.Error("expected CycloneDX content even with no components")
	}
}

func TestGenerateSPDX_Supplier(t *testing.T) {
	meta := newMeta()
	meta.Supplier = "Acme Corp"
	g := sbom.NewGenerator(meta)
	doc, _ := g.Generate(sbom.FormatSPDX)
	if !strings.Contains(doc.Content, "Creator: Organization: Acme Corp") {
		t.Error("expected Supplier in SPDX output")
	}
}

func TestGenerateCycloneDX_Timestamp(t *testing.T) {
	g := sbom.NewGenerator(newMeta())
	doc, _ := g.Generate(sbom.FormatCycloneDX)
	if !strings.Contains(doc.Content, "2026-01-15T12:00:00Z") {
		t.Error("expected fixed timestamp in CycloneDX output")
	}
}
