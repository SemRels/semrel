// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package terraform_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/terraform"
)

func writeFakeBinary(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	os.WriteFile(path, []byte("fake binary content"), 0o644)
	return path
}

func TestGenerateManifest_Basic(t *testing.T) {
	dir := t.TempDir()
	linBin := writeFakeBinary(t, dir, "terraform-provider-mycloud_1.0.0_linux_amd64.zip")
	darBin := writeFakeBinary(t, dir, "terraform-provider-mycloud_1.0.0_darwin_amd64.zip")

	g := terraform.NewManifestGenerator(terraform.ManifestConfig{
		Namespace:           "myorg",
		Type:                "mycloud",
		Version:             "1.0.0",
		DownloadURLTemplate: "https://github.com/myorg/releases/download/v1.0.0/{filename}",
	})

	platforms := []terraform.ProviderPlatform{
		{OS: "linux", Arch: "amd64", BinaryPath: linBin},
		{OS: "darwin", Arch: "amd64", BinaryPath: darBin},
	}

	m, err := g.GenerateManifest(platforms)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", m.Version)
	}
	if len(m.Platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(m.Platforms))
	}
	if m.Platforms[0].SHA256 == "" {
		t.Error("expected non-empty SHA256")
	}
	if m.Platforms[0].DownloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestGenerateManifest_DefaultProtocols(t *testing.T) {
	g := terraform.NewManifestGenerator(terraform.ManifestConfig{Version: "1.0.0"})
	m, err := g.GenerateManifest(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Protocols) == 0 || m.Protocols[0] != "5.0" {
		t.Errorf("expected default protocol 5.0, got %v", m.Protocols)
	}
}

func TestGenerateManifest_MissingFile(t *testing.T) {
	g := terraform.NewManifestGenerator(terraform.ManifestConfig{Version: "1.0.0"})
	platforms := []terraform.ProviderPlatform{
		{OS: "linux", Arch: "amd64", BinaryPath: "/nonexistent/file.zip"},
	}
	_, err := g.GenerateManifest(platforms)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestMarshalManifest_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeBinary(t, dir, "provider.zip")

	g := terraform.NewManifestGenerator(terraform.ManifestConfig{
		Version:             "2.0.0",
		DownloadURLTemplate: "https://example.com/{filename}",
	})
	m, _ := g.GenerateManifest([]terraform.ProviderPlatform{{OS: "linux", Arch: "amd64", BinaryPath: bin}})
	b, err := terraform.MarshalManifest(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if raw["version"] == nil {
		t.Error("expected version field in JSON")
	}
}

func TestWriteChecksums(t *testing.T) {
	dir := t.TempDir()
	binA := writeFakeBinary(t, dir, "provider_linux_amd64.zip")
	binB := writeFakeBinary(t, dir, "provider_darwin_amd64.zip")

	g := terraform.NewManifestGenerator(terraform.ManifestConfig{Version: "1.0.0", DownloadURLTemplate: "https://x/{filename}"})
	m, _ := g.GenerateManifest([]terraform.ProviderPlatform{
		{OS: "linux", Arch: "amd64", BinaryPath: binA},
		{OS: "darwin", Arch: "amd64", BinaryPath: binB},
	})

	var buf bytes.Buffer
	if err := terraform.WriteChecksums(&buf, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 checksum lines, got %d", len(lines))
	}
}

func TestModuleTagName(t *testing.T) {
	if got := terraform.ModuleTagName("v1.2.3"); got != "1.2.3" {
		t.Errorf("expected 1.2.3, got %q", got)
	}
	if got := terraform.ModuleTagName("1.2.3"); got != "1.2.3" {
		t.Errorf("expected 1.2.3, got %q", got)
	}
}

func TestProviderTagName(t *testing.T) {
	if got := terraform.ProviderTagName("1.2.3"); got != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %q", got)
	}
	if got := terraform.ProviderTagName("v1.2.3"); got != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %q", got)
	}
}

func TestRegistryURL(t *testing.T) {
	got := terraform.RegistryURL("hashicorp", "aws", "4.0.0")
	want := "https://registry.terraform.io/providers/hashicorp/aws/4.0.0"
	if got != want {
		t.Errorf("RegistryURL: got %q, want %q", got, want)
	}
}
