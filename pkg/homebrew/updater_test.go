// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package homebrew_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/homebrew"
)

func TestDownloadURL(t *testing.T) {
	u := homebrew.NewUpdater(homebrew.FormulaConfig{
		Name:        "semrel",
		Version:     "1.2.3",
		URLTemplate: "https://github.com/example/releases/download/v{version}/semrel_{version}_linux_amd64.tar.gz",
	})
	got := u.DownloadURL()
	want := "https://github.com/example/releases/download/v1.2.3/semrel_1.2.3_linux_amd64.tar.gz"
	if got != want {
		t.Errorf("DownloadURL: got %q, want %q", got, want)
	}
}

func TestComputeSHA256_PreSet(t *testing.T) {
	u := homebrew.NewUpdater(homebrew.FormulaConfig{
		SHA256: "deadbeef1234",
	})
	got, err := u.ComputeSHA256()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "deadbeef1234" {
		t.Errorf("expected pre-set SHA256, got %q", got)
	}
}

func TestComputeSHA256_Download(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake tarball content"))
	}))
	defer srv.Close()

	u := homebrew.NewUpdater(homebrew.FormulaConfig{
		URLTemplate: srv.URL + "/download/v{version}/app.tar.gz",
		Version:     "2.0.0",
	})
	got, err := u.ComputeSHA256()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 64 {
		t.Errorf("expected 64-char hex digest, got %d chars: %q", len(got), got)
	}
}

func TestComputeSHA256_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	u := homebrew.NewUpdater(homebrew.FormulaConfig{
		URLTemplate: srv.URL + "/{version}",
		Version:     "1.0.0",
	})
	_, err := u.ComputeSHA256()
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestUpdateFormulaFile(t *testing.T) {
	formula := `class Semrel < Formula
  url "https://example.com/old/v0.1.0/semrel.tar.gz"
  sha256 "oldhash"
  version "0.1.0"
end
`
	dir := t.TempDir()
	path := filepath.Join(dir, "semrel.rb")
	os.WriteFile(path, []byte(formula), 0o644)

	u := homebrew.NewUpdater(homebrew.FormulaConfig{
		Name:        "semrel",
		Version:     "1.0.0",
		URLTemplate: "https://example.com/new/v{version}/semrel.tar.gz",
		SHA256:      "newhash123",
	})
	if err := u.UpdateFormulaFile(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(path)
	updated := string(content)
	if !strings.Contains(updated, `url "https://example.com/new/v1.0.0/semrel.tar.gz"`) {
		t.Error("expected updated URL in formula")
	}
	if !strings.Contains(updated, `sha256 "newhash123"`) {
		t.Error("expected updated sha256 in formula")
	}
	if !strings.Contains(updated, `version "1.0.0"`) {
		t.Error("expected updated version in formula")
	}
}

func TestGenerateFormula(t *testing.T) {
	u := homebrew.NewUpdater(homebrew.FormulaConfig{
		Name:        "semrel",
		Version:     "2.0.0",
		URLTemplate: "https://example.com/v{version}/semrel.tar.gz",
		SHA256:      "abc123",
	})
	formula, err := u.GenerateFormula()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(formula, "class Semrel < Formula") {
		t.Error("expected class declaration")
	}
	if !strings.Contains(formula, `version "2.0.0"`) {
		t.Error("expected version in generated formula")
	}
	if !strings.Contains(formula, `sha256 "abc123"`) {
		t.Error("expected sha256 in generated formula")
	}
}

func TestUpdateFormulaFile_NotFound(t *testing.T) {
	u := homebrew.NewUpdater(homebrew.FormulaConfig{SHA256: "hash"})
	if err := u.UpdateFormulaFile("/nonexistent/formula.rb"); err == nil {
		t.Fatal("expected error for missing formula file")
	}
}
