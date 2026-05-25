// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package signing provides artifact signing using Cosign and SLSA provenance
// attestation for release artifacts.
//
// Cosign (https://github.com/sigstore/cosign) is the CNCF standard for signing
// container images and release artifacts using Sigstore's keyless or key-based
// approach. SLSA provenance links artifacts to the build that produced them.
//
// This package wraps the cosign and gpg CLI tools. They must be installed and
// available on PATH. In GitHub Actions, use the official cosign action:
// https://github.com/sigstore/cosign-installer
//
// Supported operations:
//   - Sign artifact files (cosign: .bundle sidecar; gpg: .asc detached sig)
//   - Verify cosign signatures
//   - Generate SLSA Level 1 build provenance (JSON)
//
// See: https://github.com/SemRels/semrel/issues/35
package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Backend selects the signing backend.
type Backend string

const (
	// BackendCosign uses the cosign CLI for signing.
	BackendCosign Backend = "cosign"
	// BackendGPG uses the gpg CLI for signing (detached .asc files).
	BackendGPG Backend = "gpg"
)

// Config holds the signing configuration.
type Config struct {
	// Backend selects cosign (default) or gpg.
	Backend Backend
	// KeyPath is the path to the cosign private key or GPG key fingerprint.
	// If empty, cosign uses keyless signing via OIDC.
	KeyPath string
	// KeyPassword is the password for the cosign private key.
	KeyPassword string
	// Annotations are extra key=value pairs attached to the cosign signature.
	Annotations map[string]string
}

// Signer signs and verifies release artifacts.
type Signer struct {
	cfg     Config
	execCmd func(name string, args ...string) *exec.Cmd
}

// NewSigner creates a Signer from the given configuration.
func NewSigner(cfg Config) *Signer {
	if cfg.Backend == "" {
		cfg.Backend = BackendCosign
	}
	return &Signer{cfg: cfg, execCmd: exec.Command}
}

// SignArtifact signs the file at path using the configured backend.
// For cosign: produces a .bundle file alongside the artifact.
// For gpg: produces a .asc (detached ASCII armored signature).
func (s *Signer) SignArtifact(path string) error {
	switch s.cfg.Backend {
	case BackendCosign:
		return s.cosignSign(path)
	case BackendGPG:
		return s.gpgSign(path)
	default:
		return fmt.Errorf("signing: unknown backend %q", s.cfg.Backend)
	}
}

// VerifyArtifact verifies a cosign signature for the file at path.
func (s *Signer) VerifyArtifact(path string) error {
	if s.cfg.Backend != BackendCosign {
		return fmt.Errorf("signing: verify not supported for backend %q", s.cfg.Backend)
	}
	return s.cosignVerify(path)
}

func (s *Signer) cosignSign(path string) error {
	args := []string{"sign-blob", "--yes"}
	if s.cfg.KeyPath != "" {
		args = append(args, "--key", s.cfg.KeyPath)
	}
	for k, v := range s.cfg.Annotations {
		args = append(args, "--annotations", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, "--bundle", path+".bundle", path)

	cmd := s.execCmd("cosign", args...)
	if s.cfg.KeyPassword != "" {
		cmd.Env = append(os.Environ(), "COSIGN_PASSWORD="+s.cfg.KeyPassword)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign sign-blob: %w\n%s", err, string(out))
	}
	return nil
}

func (s *Signer) cosignVerify(path string) error {
	args := []string{"verify-blob"}
	if s.cfg.KeyPath != "" {
		args = append(args, "--key", s.cfg.KeyPath)
	}
	args = append(args, "--bundle", path+".bundle", path)

	cmd := s.execCmd("cosign", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign verify-blob: %w\n%s", err, string(out))
	}
	return nil
}

func (s *Signer) gpgSign(path string) error {
	args := []string{"--batch", "--yes", "--armor", "--detach-sign"}
	if s.cfg.KeyPath != "" {
		args = append(args, "--local-user", s.cfg.KeyPath)
	}
	args = append(args, path)

	cmd := s.execCmd("gpg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg sign: %w\n%s", err, string(out))
	}
	return nil
}

// ---- SLSA Provenance -------------------------------------------------------

// Provenance is a SLSA Level 1 build provenance document for a release.
type Provenance struct {
	Subject    []ProvenanceSubject  `json:"subject"`
	BuildType  string               `json:"buildType"`
	Builder    ProvenanceBuilder    `json:"builder"`
	Invocation ProvenanceInvocation `json:"invocation"`
	Metadata   ProvenanceMetadata   `json:"metadata"`
}

// ProvenanceSubject is a named artifact with its SHA-256 digest.
type ProvenanceSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// ProvenanceBuilder identifies the build system.
type ProvenanceBuilder struct {
	ID string `json:"id"`
}

// ProvenanceInvocation captures the build trigger.
type ProvenanceInvocation struct {
	ConfigSource ProvenanceConfigSource `json:"configSource"`
}

// ProvenanceConfigSource is the URI of the workflow definition.
type ProvenanceConfigSource struct {
	URI string `json:"uri"`
}

// ProvenanceMetadata holds timing information.
type ProvenanceMetadata struct {
	BuildStartedOn  string                 `json:"buildStartedOn"`
	BuildFinishedOn string                 `json:"buildFinishedOn"`
	Completeness    ProvenanceCompleteness `json:"completeness"`
}

// ProvenanceCompleteness records which provenance fields are fully populated.
type ProvenanceCompleteness struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}

// GenerateProvenance creates a SLSA Level 1 provenance document.
// Each artifact file must exist; its SHA-256 digest is computed and included.
func GenerateProvenance(artifacts []string, builderID, workflowURI string) (*Provenance, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := &Provenance{
		BuildType: "https://slsa.dev/provenance/v0.2",
		Builder:   ProvenanceBuilder{ID: builderID},
		Invocation: ProvenanceInvocation{
			ConfigSource: ProvenanceConfigSource{URI: workflowURI},
		},
		Metadata: ProvenanceMetadata{
			BuildStartedOn:  now,
			BuildFinishedOn: now,
		},
	}
	for _, path := range artifacts {
		digest, err := sha256File(path)
		if err != nil {
			return nil, fmt.Errorf("provenance: digest %s: %w", path, err)
		}
		p.Subject = append(p.Subject, ProvenanceSubject{
			Name:   filepath.Base(path),
			Digest: map[string]string{"sha256": digest},
		})
	}
	return p, nil
}

// MarshalProvenance serializes a Provenance to indented JSON.
func MarshalProvenance(p *Provenance) ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// SigFiles returns the expected signature sidecar file paths for an artifact.
func SigFiles(path string, backend Backend) []string {
	switch backend {
	case BackendGPG:
		return []string{path + ".asc"}
	default:
		return []string{path + ".bundle"}
	}
}

// IsCosignAvailable returns true if the cosign binary is on PATH.
func IsCosignAvailable() bool {
	_, err := exec.LookPath("cosign")
	return err == nil
}

// IsGPGAvailable returns true if the gpg binary is on PATH.
func IsGPGAvailable() bool {
	_, err := exec.LookPath("gpg")
	return err == nil
}

// SupportedBackends returns signing backends whose CLI tools are available.
func SupportedBackends() []Backend {
	var b []Backend
	if IsCosignAvailable() {
		b = append(b, BackendCosign)
	}
	if IsGPGAvailable() {
		b = append(b, BackendGPG)
	}
	return b
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
