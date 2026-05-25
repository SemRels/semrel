// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package docker provides a Docker image tagging and push plugin.
// It wraps the Docker CLI (docker) to tag and push images to one or more
// registries without needing the Docker daemon SDK as a dependency.
package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Config holds the Docker plugin configuration.
type Config struct {
	// Image is the source image name with optional tag (e.g., "myapp:latest").
	Image string
	// Registries is the list of registry prefixes to push to
	// (e.g., "ghcr.io/myorg", "docker.io/myorg").
	Registries []string
	// Tags is the list of version tags to apply (e.g., "v1.2.3", "1.2", "1").
	Tags []string
	// Platforms is a list of platforms to build for when using buildx
	// (e.g., "linux/amd64,linux/arm64"). Empty means single-platform.
	Platforms []string
	// Labels is a map of OCI label key-value pairs to apply.
	Labels map[string]string
}

// Tagger tags and pushes Docker images to one or more registries.
type Tagger struct {
	cfg    Config
	runner cmdRunner
}

// cmdRunner abstracts exec.Command for testing.
type cmdRunner interface {
	run(ctx context.Context, name string, args ...string) error
}

type realRunner struct{}

func (realRunner) run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker: %s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

// NewTagger creates a Tagger with the provided configuration.
func NewTagger(cfg Config) *Tagger {
	return &Tagger{cfg: cfg, runner: realRunner{}}
}

// TagAndPush tags the source image with every (registry, tag) combination
// and pushes each resulting image reference. Returns all pushed references.
func (t *Tagger) TagAndPush(ctx context.Context) ([]string, error) {
	var pushed []string
	for _, registry := range t.cfg.Registries {
		for _, tag := range t.cfg.Tags {
			ref := buildRef(registry, t.cfg.Image, tag)
			if err := t.runner.run(ctx, "docker", "tag", t.cfg.Image, ref); err != nil {
				return pushed, err
			}
			if err := t.runner.run(ctx, "docker", "push", ref); err != nil {
				return pushed, err
			}
			pushed = append(pushed, ref)
		}
	}
	return pushed, nil
}

// BuildAndPush builds a multi-platform image with docker buildx and pushes it
// to all configured registries with all configured tags. The build context is
// the provided directory (usually ".").
func (t *Tagger) BuildAndPush(ctx context.Context, buildContext string) ([]string, error) {
	if len(t.cfg.Platforms) == 0 {
		return nil, fmt.Errorf("docker: BuildAndPush requires at least one platform")
	}

	var refs []string
	args := []string{"buildx", "build",
		"--platform", strings.Join(t.cfg.Platforms, ","),
		"--push",
	}

	// Add all registry/tag combinations as --tag flags
	for _, registry := range t.cfg.Registries {
		for _, tag := range t.cfg.Tags {
			ref := buildRef(registry, t.cfg.Image, tag)
			args = append(args, "--tag", ref)
			refs = append(refs, ref)
		}
	}

	// Add labels
	for k, v := range t.cfg.Labels {
		args = append(args, "--label", k+"="+v)
	}

	args = append(args, buildContext)
	if err := t.runner.run(ctx, "docker", args...); err != nil {
		return refs, err
	}
	return refs, nil
}

// GenerateTags returns the conventional set of semver tags for a version string.
// For example, "v1.2.3" yields ["v1.2.3", "v1.2", "v1", "latest"].
func GenerateTags(version string) []string {
	v := strings.TrimPrefix(version, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return []string{"v" + v, "latest"}
	}
	return []string{
		"v" + v,                          // v1.2.3
		"v" + parts[0] + "." + parts[1], // v1.2
		"v" + parts[0],                   // v1
		"latest",
	}
}

// buildRef constructs a full image reference from registry, image name, and tag.
func buildRef(registry, image, tag string) string {
	// Strip any existing tag from the image name
	name := image
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		name = name[:idx]
	}
	// Strip any existing registry prefix from the image name
	registry = strings.TrimRight(registry, "/")
	return registry + "/" + name + ":" + tag
}

// IsDockerAvailable reports whether the docker CLI is installed.
func IsDockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}
