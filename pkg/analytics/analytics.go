// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package analytics provides release metrics and history tracking for semrel.
// Metrics are stored as newline-delimited JSON in a configurable file
// (default: .semrel-analytics.jsonl) so they can be queried, grepped and
// piped into dashboards without requiring a database.
//
// Each release appends one ReleaseRecord line to the analytics file, enabling
// simple queries like:
//
//	grep '"bump":"major"' .semrel-analytics.jsonl | wc -l
//	jq 'select(.released)' .semrel-analytics.jsonl | jq '.version'
//
// See: https://github.com/SemRels/semrel/issues/50
package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DefaultFile is the default path for the analytics log.
const DefaultFile = ".semrel-analytics.jsonl"

// ReleaseRecord represents a single release event written to the analytics log.
type ReleaseRecord struct {
	// Timestamp is the UTC time of the release in RFC 3339 format.
	Timestamp string `json:"timestamp"`
	// Version is the new release version (e.g. "1.2.0").
	Version string `json:"version"`
	// PreviousVersion is the version before this release.
	PreviousVersion string `json:"previous_version"`
	// Bump is the semver bump type: major, minor, patch.
	Bump string `json:"bump"`
	// Commits is the total number of commits in this release.
	Commits int `json:"commits"`
	// Breaking indicates whether any breaking change commits were included.
	Breaking bool `json:"breaking"`
	// Features indicates whether any feature commits were included.
	Features bool `json:"features"`
	// Fixes indicates whether any fix commits were included.
	Fixes bool `json:"fixes"`
	// DryRun is true when the record was created during a dry-run.
	DryRun bool `json:"dry_run,omitempty"`
	// Branch is the git branch from which the release was made.
	Branch string `json:"branch,omitempty"`
}

// Tracker writes and reads release analytics records.
type Tracker struct {
	path string
}

// NewTracker creates a tracker that writes to the given file path.
// Use DefaultFile for the standard location.
func NewTracker(path string) *Tracker {
	return &Tracker{path: path}
}

// Record appends a ReleaseRecord to the analytics log file.
// The file is created if it does not exist. Each record is written as a single
// JSON line (NDJSON) so the file can be incrementally queried.
func (t *Tracker) Record(r ReleaseRecord) error {
	if r.Timestamp == "" {
		r.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening analytics file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	line, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshalling record: %w", err)
	}
	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}

// ReadAll reads all release records from the analytics log file.
// Returns an empty slice (no error) if the file does not exist yet.
func (t *Tracker) ReadAll() ([]ReleaseRecord, error) {
	f, err := os.Open(t.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening analytics file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var records []ReleaseRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r ReleaseRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("parsing record: %w", err)
		}
		records = append(records, r)
	}
	return records, scanner.Err()
}

// Summary computes aggregate metrics over all recorded releases.
type Summary struct {
	// TotalReleases is the total number of release records.
	TotalReleases int `json:"total_releases"`
	// MajorBumps is the count of major version bumps.
	MajorBumps int `json:"major_bumps"`
	// MinorBumps is the count of minor version bumps.
	MinorBumps int `json:"minor_bumps"`
	// PatchBumps is the count of patch version bumps.
	PatchBumps int `json:"patch_bumps"`
	// BreakingReleases is the count of releases containing breaking changes.
	BreakingReleases int `json:"breaking_releases"`
	// AverageCommitsPerRelease is the mean commit count per release.
	AverageCommitsPerRelease float64 `json:"avg_commits_per_release"`
	// LatestVersion is the version string from the most recent record.
	LatestVersion string `json:"latest_version,omitempty"`
}

// Summarise computes aggregate metrics from the given records.
func Summarise(records []ReleaseRecord) Summary {
	if len(records) == 0 {
		return Summary{}
	}
	s := Summary{TotalReleases: len(records)}
	totalCommits := 0
	for _, r := range records {
		switch r.Bump {
		case "major":
			s.MajorBumps++
		case "minor":
			s.MinorBumps++
		case "patch":
			s.PatchBumps++
		}
		if r.Breaking {
			s.BreakingReleases++
		}
		totalCommits += r.Commits
	}
	s.AverageCommitsPerRelease = float64(totalCommits) / float64(len(records))
	s.LatestVersion = records[len(records)-1].Version
	return s
}
