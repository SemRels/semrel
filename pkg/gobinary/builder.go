// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package gobinary provides cross-compilation and release asset packaging for
// Go binaries. It wraps "go build" for multiple GOOS/GOARCH targets, computes
// SHA-256 checksums, and creates compressed release archives.
package gobinary

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Target represents a GOOS/GOARCH build target.
type Target struct {
	OS   string
	Arch string
}

// String returns "os_arch" identifier.
func (t Target) String() string {
	return t.OS + "_" + t.Arch
}

// DefaultTargets is the standard set of release targets.
var DefaultTargets = []Target{
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
	{OS: "windows", Arch: "amd64"},
}

// BuildConfig holds the configuration for a cross-compile build.
type BuildConfig struct {
	// MainPackage is the Go package path to compile (e.g., "./cmd/mytool").
	MainPackage string
	// BinaryName is the base name of the output binary (without extension).
	BinaryName string
	// Version is the release version string, injected via ldflags if LDFlagVar is set.
	Version string
	// LDFlagVar is the full package-qualified variable for version injection,
	// e.g., "github.com/example/myapp/cmd.Version".
	LDFlagVar string
	// OutputDir is the directory where archives and checksums are written.
	OutputDir string
	// Targets is the list of OS/arch targets to build. Defaults to DefaultTargets.
	Targets []Target
	// CGOEnabled controls whether CGO is enabled (default: disabled for portability).
	CGOEnabled bool
}

// Artifact describes a build output.
type Artifact struct {
	// Target is the OS/arch pair this artifact was built for.
	Target Target
	// ArchivePath is the path to the .tar.gz or .zip archive.
	ArchivePath string
	// Checksum is the SHA-256 hex digest of the archive.
	Checksum string
}

// Builder cross-compiles a Go binary for multiple platforms.
type Builder struct {
	cfg BuildConfig
}

// NewBuilder creates a Builder with the provided configuration.
func NewBuilder(cfg BuildConfig) *Builder {
	if len(cfg.Targets) == 0 {
		cfg.Targets = DefaultTargets
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "dist"
	}
	return &Builder{cfg: cfg}
}

// Build cross-compiles the binary for all configured targets and returns the
// list of Artifacts (archives + checksums). It also writes a SHA256SUMS file
// in the output directory.
func (b *Builder) Build() ([]Artifact, error) {
	if err := os.MkdirAll(b.cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("gobinary: create output dir: %w", err)
	}

	var artifacts []Artifact
	for _, target := range b.cfg.Targets {
		a, err := b.buildTarget(target)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}

	if err := b.writeChecksums(artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (b *Builder) buildTarget(t Target) (Artifact, error) {
	// Determine binary filename
	binName := b.cfg.BinaryName
	if t.OS == "windows" {
		binName += ".exe"
	}

	// Build to a temp file
	tmpBin := filepath.Join(b.cfg.OutputDir, binName+"."+t.String()+".tmp")
	defer os.Remove(tmpBin)

	args := []string{"build", "-o", tmpBin}
	if b.cfg.LDFlagVar != "" && b.cfg.Version != "" {
		args = append(args, "-ldflags",
			fmt.Sprintf("-X %s=%s", b.cfg.LDFlagVar, b.cfg.Version))
	}
	args = append(args, b.cfg.MainPackage)

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(),
		"GOOS="+t.OS,
		"GOARCH="+t.Arch,
	)
	if !b.cfg.CGOEnabled {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return Artifact{}, fmt.Errorf("gobinary: build %s: %w\n%s", t, err, out)
	}

	// Create archive
	archivePath, err := b.createArchive(tmpBin, binName, t)
	if err != nil {
		return Artifact{}, err
	}

	// Compute SHA-256
	chk, err := sha256File(archivePath)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{Target: t, ArchivePath: archivePath, Checksum: chk}, nil
}

func (b *Builder) createArchive(binPath, binName string, t Target) (string, error) {
	dirName := fmt.Sprintf("%s_%s_%s", b.cfg.BinaryName, b.cfg.Version, t)
	if t.OS == "windows" {
		return b.createZip(binPath, binName, dirName)
	}
	return b.createTarGz(binPath, binName, dirName)
}

func (b *Builder) createTarGz(binPath, binName, dirName string) (string, error) {
	archivePath := filepath.Join(b.cfg.OutputDir, dirName+".tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("gobinary: create archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	src, err := os.Open(binPath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	info, _ := src.Stat()

	hdr := &tar.Header{
		Name: dirName + "/" + binName,
		Mode: 0o755,
		Size: info.Size(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return "", err
	}
	if _, err := io.Copy(tw, src); err != nil {
		return "", err
	}
	return archivePath, nil
}

func (b *Builder) createZip(binPath, binName, dirName string) (string, error) {
	archivePath := filepath.Join(b.cfg.OutputDir, dirName+".zip")
	f, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("gobinary: create zip: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	src, err := os.Open(binPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	w, err := zw.Create(dirName + "/" + binName)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(w, src); err != nil {
		return "", err
	}
	return archivePath, nil
}

func (b *Builder) writeChecksums(artifacts []Artifact) error {
	var sb strings.Builder
	for _, a := range artifacts {
		sb.WriteString(fmt.Sprintf("%s  %s\n", a.Checksum, filepath.Base(a.ArchivePath)))
	}
	sumPath := filepath.Join(b.cfg.OutputDir, "SHA256SUMS")
	return os.WriteFile(sumPath, []byte(sb.String()), 0o644)
}

// SHA256File computes the SHA-256 checksum of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HostTarget returns the Target matching the current OS/arch.
func HostTarget() Target {
	return Target{OS: runtime.GOOS, Arch: runtime.GOARCH}
}
