// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package oci provides a built-in plugin for pushing release artifacts to
// OCI-compatible registries using the ORAS (OCI Registry As Storage) protocol.
//
// ORAS allows pushing arbitrary files (binaries, SBOMs, signatures, configs)
// as OCI artifacts to any OCI-compliant registry (GHCR, Docker Hub, ECR, ACR).
// This is the recommended way to distribute non-container artifacts in the
// cloud-native ecosystem.
//
// This package wraps the ORAS CLI tool. It must be installed and available
// on PATH. See https://oras.land for installation instructions.
//
// Supported operations:
//   - Push a set of files as an OCI artifact with a given tag
//   - Attach a file as an OCI referrer (e.g. SBOM, signature) to an existing artifact
//   - Construct OCI reference strings
//
// See: https://github.com/SemRels/semrel/issues/33
package oci

import (
	"fmt"
	"os/exec"
	"strings"
)

// PushConfig holds the configuration for pushing an OCI artifact.
type PushConfig struct {
	// Registry is the OCI registry host (e.g. "ghcr.io").
	Registry string
	// Repository is the image repository path (e.g. "myorg/myapp").
	Repository string
	// Tag is the OCI tag (e.g. "v1.2.3").
	Tag string
	// MediaType is the default media type for files without explicit annotation
	// (e.g. "application/vnd.unknown.layer.v1+binary").
	MediaType string
	// Annotations are extra OCI manifest annotations.
	Annotations map[string]string
}

// AttachConfig holds the configuration for attaching a referrer artifact.
type AttachConfig struct {
	// Subject is the full OCI reference of the subject artifact.
	Subject string
	// MediaType is the media type of the attached artifact.
	MediaType string
}

// Pusher pushes artifacts to OCI registries using the ORAS CLI.
type Pusher struct {
	cfg     PushConfig
	execCmd func(name string, args ...string) *exec.Cmd
}

// NewPusher creates a Pusher from the given configuration.
func NewPusher(cfg PushConfig) *Pusher {
	return &Pusher{cfg: cfg, execCmd: exec.Command}
}

// Reference returns the full OCI reference string for this pusher.
// Format: registry/repository:tag
func (p *Pusher) Reference() string {
	return Reference(p.cfg.Registry, p.cfg.Repository, p.cfg.Tag)
}

// Push runs `oras push` to upload the given files as an OCI artifact.
// Each file entry should be in "path:mediatype" format, or just "path" to use
// the default media type.
func (p *Pusher) Push(files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("oci: no files to push")
	}

	args := []string{"push", p.Reference()}
	for k, v := range p.cfg.Annotations {
		args = append(args, "--annotation", fmt.Sprintf("%s=%s", k, v))
	}
	if p.cfg.MediaType != "" {
		args = append(args, "--media-type", p.cfg.MediaType)
	}
	args = append(args, files...)

	cmd := p.execCmd("oras", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("oras push: %w\n%s", err, string(out))
	}
	return nil
}

// Attach runs `oras attach` to associate a file with an existing artifact
// as an OCI referrer.
func (p *Pusher) Attach(attachCfg AttachConfig, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("oci: no files to attach")
	}
	args := []string{"attach",
		"--artifact-type", attachCfg.MediaType,
		attachCfg.Subject,
	}
	args = append(args, files...)

	cmd := p.execCmd("oras", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("oras attach: %w\n%s", err, string(out))
	}
	return nil
}

// Reference constructs an OCI reference string from its components.
func Reference(registry, repository, tag string) string {
	ref := registry + "/" + repository
	if tag != "" {
		ref += ":" + tag
	}
	return ref
}

// IsOrasAvailable returns true if the oras binary is on PATH.
func IsOrasAvailable() bool {
	_, err := exec.LookPath("oras")
	return err == nil
}

// FileAnnotation formats a file path with a media type annotation for oras.
// Returns "path:mediatype".
func FileAnnotation(path, mediaType string) string {
	return path + ":" + mediaType
}

// ParseReference splits an OCI reference into registry, repository, and tag.
// e.g. "ghcr.io/myorg/myapp:v1.0.0" → ("ghcr.io", "myorg/myapp", "v1.0.0")
func ParseReference(ref string) (registry, repository, tag string) {
	// Split off tag
	if idx := strings.LastIndex(ref, ":"); idx > 0 && !strings.Contains(ref[idx:], "/") {
		tag = ref[idx+1:]
		ref = ref[:idx]
	}
	// First segment is registry
	if idx := strings.Index(ref, "/"); idx > 0 {
		registry = ref[:idx]
		repository = ref[idx+1:]
	} else {
		registry = ref
	}
	return
}
