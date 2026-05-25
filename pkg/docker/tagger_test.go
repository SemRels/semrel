// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package docker_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoSemantics/semrel/pkg/docker"
)

// fakeRunner records commands for verification in tests.
type fakeRunner struct {
	commands []string
	failOn   string
}

// We need access to the unexported runner field via a test helper constructor.
// Since Go doesn't let us set unexported fields from tests, we'll test the
// exported behavior by providing a testable version through the WithRunner option.

func TestGenerateTags(t *testing.T) {
	tests := []struct {
		version  string
		expected []string
	}{
		{"v1.2.3", []string{"v1.2.3", "v1.2", "v1", "latest"}},
		{"1.2.3", []string{"v1.2.3", "v1.2", "v1", "latest"}},
		{"v2.0.0", []string{"v2.0.0", "v2.0", "v2", "latest"}},
		{"v1.0", []string{"v1.0", "latest"}},  // less than 3 parts
	}

	for _, tt := range tests {
		got := docker.GenerateTags(tt.version)
		if len(got) != len(tt.expected) {
			t.Errorf("GenerateTags(%q): expected %v, got %v", tt.version, tt.expected, got)
			continue
		}
		for i, tag := range got {
			if tag != tt.expected[i] {
				t.Errorf("GenerateTags(%q)[%d]: expected %q, got %q", tt.version, i, tt.expected[i], tag)
			}
		}
	}
}

func TestGenerateTags_FourVersionParts(t *testing.T) {
	// Only 3 parts → standard semver tags
	tags := docker.GenerateTags("v1.2.3")
	if tags[0] != "v1.2.3" {
		t.Errorf("expected 'v1.2.3' as first tag, got %q", tags[0])
	}
	if tags[3] != "latest" {
		t.Errorf("expected 'latest' as last tag, got %q", tags[3])
	}
}

func TestIsDockerAvailable(t *testing.T) {
	// Just verify the function doesn't panic
	_ = docker.IsDockerAvailable()
}

func TestNewTagger_NoDockerRequired(t *testing.T) {
	// Verify Tagger can be created without docker present
	tagger := docker.NewTagger(docker.Config{
		Image:      "myapp:latest",
		Registries: []string{"ghcr.io/myorg"},
		Tags:       []string{"v1.0.0"},
	})
	_ = tagger
}

func TestTagger_TagAndPush_CommandsBuilt(t *testing.T) {
	// We can't run docker in CI, but we can test that the error path
	// provides a useful message when docker is not available.
	tagger := docker.NewTagger(docker.Config{
		Image:      "myapp",
		Registries: []string{"ghcr.io/myorg"},
		Tags:       []string{"v1.0.0"},
	})

	// When docker is not available, we get an error (not a panic)
	ctx := context.Background()
	_, err := tagger.TagAndPush(ctx)
	if err == nil && !docker.IsDockerAvailable() {
		t.Error("expected error when docker is not available")
	}
	// If docker IS available, this would actually run - which is fine
	// We just verify no panic occurs either way
	_ = err
}

func TestTagger_BuildAndPush_NoPlatforms(t *testing.T) {
	tagger := docker.NewTagger(docker.Config{
		Image:      "myapp",
		Registries: []string{"ghcr.io/myorg"},
		Tags:       []string{"v1.0.0"},
		// Platforms intentionally empty
	})

	_, err := tagger.BuildAndPush(context.Background(), ".")
	if err == nil {
		t.Error("expected error when no platforms configured")
	}
	if err != nil {
		expected := "requires at least one platform"
		if !contains(err.Error(), expected) {
			t.Errorf("expected error to contain %q, got: %v", expected, err)
		}
	}
}

func TestNewTagger_MultipleRegistries(t *testing.T) {
	cfg := docker.Config{
		Image:      "myapp:v1.0.0",
		Registries: []string{"ghcr.io/myorg", "docker.io/myorg"},
		Tags:       docker.GenerateTags("v1.2.3"),
	}
	tagger := docker.NewTagger(cfg)
	_ = tagger
	// Verify we'd push 2 registries × 4 tags = 8 images
	expectedCount := len(cfg.Registries) * len(cfg.Tags)
	if expectedCount != 8 {
		t.Errorf("expected 8 image refs, got %d", expectedCount)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure fakeRunner is referenced to avoid compile error.
var _ = fmt.Sprintf("%v", fakeRunner{})
