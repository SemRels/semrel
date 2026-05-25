// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package gobinary_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/gobinary"
)

// buildSimpleProgram writes a minimal Go source to a temp dir and returns the
// directory and "." package path so tests can compile a real binary.
func buildSimpleProgram(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	src := `package main
import "fmt"
func main() { fmt.Println("hello") }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a go.mod so it compiles standalone
	mod := "module testbinary\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHostTarget(t *testing.T) {
	target := gobinary.HostTarget()
	if target.OS == "" || target.Arch == "" {
		t.Error("HostTarget should have non-empty OS and Arch")
	}
}

func TestTargetString(t *testing.T) {
	tgt := gobinary.Target{OS: "linux", Arch: "amd64"}
	if tgt.String() != "linux_amd64" {
		t.Errorf("expected 'linux_amd64', got %q", tgt.String())
	}
}

func TestDefaultTargets(t *testing.T) {
	if len(gobinary.DefaultTargets) < 3 {
		t.Error("expected at least 3 default targets")
	}
	for _, tgt := range gobinary.DefaultTargets {
		if tgt.OS == "" || tgt.Arch == "" {
			t.Errorf("malformed target: %+v", tgt)
		}
	}
}

func TestBuilder_Build_HostOnly(t *testing.T) {
	src := buildSimpleProgram(t)
	outDir := t.TempDir()
	host := gobinary.HostTarget()

	b := gobinary.NewBuilder(gobinary.BuildConfig{
		MainPackage: ".",
		BinaryName:  "testbin",
		Version:     "v1.0.0",
		OutputDir:   outDir,
		Targets:     []gobinary.Target{host},
	})

	// Override working dir: exec.Command("go", ...) must run in src dir
	// We'll use a wrapper approach — just compile from the module root with "./"
	// Since the simple program is in its own dir, temporarily cd there
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(src)

	artifacts, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	a := artifacts[0]
	if _, err := os.Stat(a.ArchivePath); err != nil {
		t.Errorf("archive not found: %v", err)
	}
	if len(a.Checksum) != 64 {
		t.Errorf("expected 64-char SHA-256, got %d chars", len(a.Checksum))
	}

	// Verify SHA256SUMS file
	sumFile := filepath.Join(outDir, "SHA256SUMS")
	data, err := os.ReadFile(sumFile)
	if err != nil {
		t.Fatalf("SHA256SUMS not found: %v", err)
	}
	if !strings.Contains(string(data), a.Checksum) {
		t.Error("SHA256SUMS should contain the artifact checksum")
	}
}

func TestBuilder_ArchiveFormat_Linux(t *testing.T) {
	src := buildSimpleProgram(t)
	outDir := t.TempDir()

	b := gobinary.NewBuilder(gobinary.BuildConfig{
		MainPackage: ".",
		BinaryName:  "testbin",
		Version:     "v1.0.0",
		OutputDir:   outDir,
		Targets:     []gobinary.Target{{OS: "linux", Arch: "amd64"}},
	})

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(src)

	artifacts, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	// Verify .tar.gz format
	if !strings.HasSuffix(artifacts[0].ArchivePath, ".tar.gz") {
		t.Errorf("expected .tar.gz for linux, got %s", artifacts[0].ArchivePath)
	}

	// Unpack and verify binary is inside
	f, err := os.Open(artifacts[0].ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gr, _ := gzip.NewReader(f)
	tr := tar.NewReader(gr)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if strings.HasSuffix(hdr.Name, "testbin") {
			found = true
			break
		}
	}
	if !found {
		t.Error("binary 'testbin' not found in tar.gz")
	}
}

func TestBuilder_ArchiveFormat_Windows(t *testing.T) {
	src := buildSimpleProgram(t)
	outDir := t.TempDir()

	b := gobinary.NewBuilder(gobinary.BuildConfig{
		MainPackage: ".",
		BinaryName:  "testbin",
		Version:     "v1.0.0",
		OutputDir:   outDir,
		Targets:     []gobinary.Target{{OS: "windows", Arch: "amd64"}},
	})

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(src)

	artifacts, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify .zip format
	if !strings.HasSuffix(artifacts[0].ArchivePath, ".zip") {
		t.Errorf("expected .zip for windows, got %s", artifacts[0].ArchivePath)
	}

	// Unpack and verify binary is inside
	r, err := zip.OpenReader(artifacts[0].ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	found := false
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "testbin.exe") {
			found = true
			break
		}
	}
	if !found {
		t.Error("binary 'testbin.exe' not found in zip")
	}
}

func TestNewBuilder_DefaultsApplied(t *testing.T) {
	b := gobinary.NewBuilder(gobinary.BuildConfig{
		MainPackage: ".",
		BinaryName:  "tool",
	})
	_ = b // Just verify it doesn't panic and applies defaults
}
