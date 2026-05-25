// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package analytics

import (
	"os"
	"path/filepath"
	"testing"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "analytics.jsonl")
}

func TestTracker_Record_CreatesFile(t *testing.T) {
	path := tempPath(t)
	tr := NewTracker(path)

	if err := tr.Record(ReleaseRecord{Version: "1.0.0", Bump: "minor"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("analytics file not created: %v", err)
	}
}

func TestTracker_Record_Appends(t *testing.T) {
	path := tempPath(t)
	tr := NewTracker(path)

	records := []ReleaseRecord{
		{Version: "1.0.0", Bump: "minor"},
		{Version: "1.1.0", Bump: "minor"},
		{Version: "2.0.0", Bump: "major", Breaking: true},
	}
	for _, r := range records {
		if err := tr.Record(r); err != nil {
			t.Fatalf("Record(%s): %v", r.Version, err)
		}
	}

	all, err := tr.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 records, got %d", len(all))
	}
	if all[2].Version != "2.0.0" {
		t.Errorf("expected last version=2.0.0, got %q", all[2].Version)
	}
}

func TestTracker_ReadAll_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.jsonl")
	tr := NewTracker(path)

	records, err := tr.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records for missing file, got %v", records)
	}
}

func TestTracker_Record_AutoTimestamp(t *testing.T) {
	path := tempPath(t)
	tr := NewTracker(path)
	tr.Record(ReleaseRecord{Version: "1.0.0"}) // no timestamp

	records, _ := tr.ReadAll()
	if len(records) == 0 {
		t.Fatal("expected one record")
	}
	if records[0].Timestamp == "" {
		t.Error("expected auto-populated Timestamp")
	}
}

func TestSummarise_Empty(t *testing.T) {
	s := Summarise(nil)
	if s.TotalReleases != 0 {
		t.Errorf("expected 0 total releases, got %d", s.TotalReleases)
	}
}

func TestSummarise_Mixed(t *testing.T) {
	records := []ReleaseRecord{
		{Version: "1.0.0", Bump: "minor", Commits: 3},
		{Version: "1.1.0", Bump: "minor", Commits: 5},
		{Version: "2.0.0", Bump: "major", Commits: 10, Breaking: true},
		{Version: "2.0.1", Bump: "patch", Commits: 1},
	}

	s := Summarise(records)

	if s.TotalReleases != 4 {
		t.Errorf("expected TotalReleases=4, got %d", s.TotalReleases)
	}
	if s.MajorBumps != 1 {
		t.Errorf("expected MajorBumps=1, got %d", s.MajorBumps)
	}
	if s.MinorBumps != 2 {
		t.Errorf("expected MinorBumps=2, got %d", s.MinorBumps)
	}
	if s.PatchBumps != 1 {
		t.Errorf("expected PatchBumps=1, got %d", s.PatchBumps)
	}
	if s.BreakingReleases != 1 {
		t.Errorf("expected BreakingReleases=1, got %d", s.BreakingReleases)
	}
	expected := float64(3+5+10+1) / 4
	if s.AverageCommitsPerRelease != expected {
		t.Errorf("expected avg=%.2f, got %.2f", expected, s.AverageCommitsPerRelease)
	}
	if s.LatestVersion != "2.0.1" {
		t.Errorf("expected LatestVersion=2.0.1, got %q", s.LatestVersion)
	}
}

func TestTracker_Record_InvalidPath(t *testing.T) {
	tr := NewTracker("/nonexistent/path/analytics.jsonl")
	err := tr.Record(ReleaseRecord{Version: "1.0.0"})
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
