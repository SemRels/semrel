// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package cioutput writes release metadata to CI-system-native output formats.
package cioutput

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReleaseMeta holds the data written to CI outputs.
type ReleaseMeta struct {
	Released        bool   `json:"released"`
	DryRun          bool   `json:"dry_run"`
	Version         string `json:"version,omitempty"`
	Tag             string `json:"tag,omitempty"`
	Bump            string `json:"bump,omitempty"`
	PreviousVersion string `json:"previous_version,omitempty"`
	Changelog       string `json:"changelog,omitempty"`
	Branch          string `json:"branch,omitempty"`
	CeilingApplied  bool   `json:"ceiling_applied,omitempty"`
}

// WriteGitHubOutput writes release metadata to $GITHUB_OUTPUT in multiline format.
func WriteGitHubOutput(meta ReleaseMeta) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return fmt.Errorf("GITHUB_OUTPUT is not set")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening GITHUB_OUTPUT: %w", err)
	}
	defer f.Close()

	lines := []string{
		"version=" + meta.Version,
		"tag=" + meta.Tag,
		"bump=" + meta.Bump,
		"previous_version=" + meta.PreviousVersion,
		"released=" + strconv.FormatBool(meta.Released),
		"dry_run=" + strconv.FormatBool(meta.DryRun),
		"branch=" + meta.Branch,
		"ceiling_applied=" + strconv.FormatBool(meta.CeilingApplied),
		"changelog<<SEMREL_EOF",
		meta.Changelog,
		"SEMREL_EOF",
	}
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	return err
}

// WriteGitLabDotenv writes a dotenv artifact file for GitLab CI.
func WriteGitLabDotenv(meta ReleaseMeta, path string) error {
	lines := []string{
		"VERSION=" + sanitizeDotenvValue(meta.Version),
		"TAG=" + sanitizeDotenvValue(meta.Tag),
		"BUMP=" + sanitizeDotenvValue(meta.Bump),
		"PREVIOUS_VERSION=" + sanitizeDotenvValue(meta.PreviousVersion),
		"RELEASED=" + strconv.FormatBool(meta.Released),
		"DRY_RUN=" + strconv.FormatBool(meta.DryRun),
		"BRANCH=" + sanitizeDotenvValue(meta.Branch),
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// WriteOutputFile writes to a file: JSON if path ends in .json, dotenv otherwise.
func WriteOutputFile(meta ReleaseMeta, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		data, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling JSON output: %w", err)
		}
		return os.WriteFile(path, append(data, '\n'), 0o644)
	}
	return WriteGitLabDotenv(meta, path)
}

func sanitizeDotenvValue(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
