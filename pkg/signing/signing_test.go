// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package signing_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/signing"
)

// fakeExec returns a function that records the last command invoked and
// optionally returns an error.
func fakeExec(t *testing.T, wantName string, failWith error) (func(string, ...string) *exec.Cmd, *[]string) {
	t.Helper()
	var captured []string
	fn := func(name string, args ...string) *exec.Cmd {
		captured = append([]string{name}, args...)
		if failWith != nil {
			// Return a command that will definitely fail
			return exec.Command("false")
		}
		// Return a no-op command
		return exec.Command(findTrueCmd())
	}
	return fn, &captured
}

// findTrueCmd returns a command that always exits 0.
func findTrueCmd() string {
	if _, err := exec.LookPath("true"); err == nil {
		return "true"
	}
	// Windows fallback
	return "cmd"
}

func TestNewSigner_DefaultBackend(t *testing.T) {
	s := signing.NewSigner(signing.Config{})
	if s == nil {
		t.Fatal("expected non-nil signer")
	}
}

func TestSignArtifact_UnknownBackend(t *testing.T) {
	s := signing.NewSigner(signing.Config{Backend: "unknown"})
	err := s.SignArtifact("/tmp/file")
	if err == nil || !strings.Contains(err.Error(), "unknown backend") {
		t.Errorf("expected unknown backend error, got: %v", err)
	}
}

func TestVerifyArtifact_NonCosignBackend(t *testing.T) {
	s := signing.NewSigner(signing.Config{Backend: signing.BackendGPG})
	err := s.VerifyArtifact("/tmp/file")
	if err == nil {
		t.Fatal("expected error for verify with GPG backend")
	}
}

func TestSigFiles_Cosign(t *testing.T) {
	files := signing.SigFiles("/dist/app-v1.tar.gz", signing.BackendCosign)
	if len(files) != 1 || files[0] != "/dist/app-v1.tar.gz.bundle" {
		t.Errorf("unexpected sig files: %v", files)
	}
}

func TestSigFiles_GPG(t *testing.T) {
	files := signing.SigFiles("/dist/app-v1.tar.gz", signing.BackendGPG)
	if len(files) != 1 || files[0] != "/dist/app-v1.tar.gz.asc" {
		t.Errorf("unexpected sig files: %v", files)
	}
}

func TestGenerateProvenance_Basic(t *testing.T) {
	dir := t.TempDir()
	art := filepath.Join(dir, "app-v1.tar.gz")
	os.WriteFile(art, []byte("fake release content"), 0o644)

	p, err := signing.GenerateProvenance([]string{art}, "https://example.com/builder", "https://github.com/SemRels/semrel/.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Subject) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(p.Subject))
	}
	if p.Subject[0].Name != "app-v1.tar.gz" {
		t.Errorf("expected filename as subject name, got %q", p.Subject[0].Name)
	}
	if p.Subject[0].Digest["sha256"] == "" {
		t.Error("expected non-empty sha256 digest")
	}
	if p.BuildType != "https://slsa.dev/provenance/v0.2" {
		t.Errorf("unexpected build type: %q", p.BuildType)
	}
}

func TestGenerateProvenance_MissingFile(t *testing.T) {
	_, err := signing.GenerateProvenance([]string{"/nonexistent/file.tar.gz"}, "builder", "workflow")
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
}

func TestMarshalProvenance_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	art := filepath.Join(dir, "binary")
	os.WriteFile(art, []byte("data"), 0o644)

	p, _ := signing.GenerateProvenance([]string{art}, "builder-id", "workflow-uri")
	b, err := signing.MarshalProvenance(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if raw["buildType"] == nil {
		t.Error("expected buildType in JSON output")
	}
}

func TestSupportedBackends_ReturnsSlice(t *testing.T) {
	// Just verify it doesn't panic and returns a slice
	backends := signing.SupportedBackends()
	for _, b := range backends {
		if b != signing.BackendCosign && b != signing.BackendGPG {
			t.Errorf("unexpected backend: %q", b)
		}
	}
}

func TestIsCosignAvailable_Bool(t *testing.T) {
	// Just verify it returns a boolean without panic
	_ = signing.IsCosignAvailable()
}
