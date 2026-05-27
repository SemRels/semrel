// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cioutput

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGitHubOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github_output.txt")
	t.Setenv("GITHUB_OUTPUT", path)

	meta := ReleaseMeta{
		Released:        true,
		DryRun:          false,
		Version:         "1.4.0",
		Tag:             "v1.4.0",
		Bump:            "minor",
		PreviousVersion: "1.3.0",
		Changelog:       "## What's Changed\n- Added thing",
		Branch:          "main",
		CeilingApplied:  false,
	}
	if err := WriteGitHubOutput(meta); err != nil {
		t.Fatalf("WriteGitHubOutput() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"version=1.4.0",
		"tag=v1.4.0",
		"bump=minor",
		"previous_version=1.3.0",
		"released=true",
		"dry_run=false",
		"branch=main",
		"ceiling_applied=false",
		"changelog<<SEMREL_EOF",
		"## What's Changed",
		"SEMREL_EOF",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestWriteGitLabDotenv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitlab.env")
	meta := ReleaseMeta{
		Released:        true,
		DryRun:          false,
		Version:         "1.4.0",
		Tag:             "v1.4.0",
		Bump:            "minor",
		PreviousVersion: "1.3.0",
		Branch:          "main",
		Changelog:       strings.Repeat("x", 1200),
	}

	if err := WriteGitLabDotenv(meta, path); err != nil {
		t.Fatalf("WriteGitLabDotenv() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"VERSION=1.4.0",
		"TAG=v1.4.0",
		"BUMP=minor",
		"PREVIOUS_VERSION=1.3.0",
		"RELEASED=true",
		"DRY_RUN=false",
		"BRANCH=main",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
	if strings.Contains(got, "CHANGELOG") {
		t.Fatalf("did not expect changelog in dotenv output: %q", got)
	}
}

func TestWriteOutputFileJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.json")
	meta := ReleaseMeta{
		Released:        true,
		DryRun:          true,
		Version:         "1.4.0",
		Tag:             "v1.4.0",
		Bump:            "minor",
		PreviousVersion: "1.3.0",
		Branch:          "main",
		CeilingApplied:  true,
	}

	if err := WriteOutputFile(meta, path); err != nil {
		t.Fatalf("WriteOutputFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got ReleaseMeta
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Tag != "v1.4.0" || !got.CeilingApplied || !got.DryRun {
		t.Fatalf("unexpected JSON output: %+v", got)
	}
}

func TestWriteOutputFileDotenv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.env")
	meta := ReleaseMeta{
		Released:        true,
		DryRun:          false,
		Version:         "1.4.0",
		Tag:             "v1.4.0",
		Bump:            "minor",
		PreviousVersion: "1.3.0",
		Branch:          "main",
	}

	if err := WriteOutputFile(meta, path); err != nil {
		t.Fatalf("WriteOutputFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "VERSION=1.4.0") || !strings.Contains(got, "TAG=v1.4.0") {
		t.Fatalf("unexpected dotenv output: %q", got)
	}
}

func TestWriteGitHubOutputReleasedFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github_output_false.txt")
	t.Setenv("GITHUB_OUTPUT", path)

	if err := WriteGitHubOutput(ReleaseMeta{Released: false, DryRun: true}); err != nil {
		t.Fatalf("WriteGitHubOutput() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "released=false") {
		t.Fatalf("expected released=false in %q", got)
	}
}
