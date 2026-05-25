// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package releasenotes

import (
	"strings"
	"testing"
	"time"
)

func testReleases() []ReleaseNotes {
	return []ReleaseNotes{
		{
			Version: "v2.0.0",
			Date:    time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
			Breaking: []Entry{
				{Type: "feat", Description: "remove legacy API", IsBreaking: true},
			},
		},
		{
			Version: "v1.2.0",
			Date:    time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
			Features: []Entry{
				{Type: "feat", Description: "add webhook support"},
			},
			Fixes: []Entry{
				{Type: "fix", Description: "fix panic on nil input"},
			},
		},
	}
}

func TestRenderRSS_ValidXML(t *testing.T) {
	cfg := DefaultFeedConfig("https://github.com/test/repo")
	cfg.Title = "Test Releases"
	cfg.Description = "Test project releases"

	out, err := RenderRSS(testReleases(), cfg)
	if err != nil {
		t.Fatalf("RenderRSS: %v", err)
	}

	if !strings.HasPrefix(out, "<?xml") {
		t.Error("expected XML declaration at start")
	}
	if !strings.Contains(out, "<rss") {
		t.Error("missing <rss> element")
	}
	if !strings.Contains(out, "version=\"2.0\"") {
		t.Error("missing RSS 2.0 version attribute")
	}
	if !strings.Contains(out, "Test Releases") {
		t.Error("missing feed title")
	}
	if !strings.Contains(out, "Release v2.0.0") {
		t.Error("missing item for v2.0.0")
	}
	if !strings.Contains(out, "Release v1.2.0") {
		t.Error("missing item for v1.2.0")
	}
	if !strings.Contains(out, "releases/tag/v2.0.0") {
		t.Error("missing release URL")
	}
}

func TestRenderRSS_Empty(t *testing.T) {
	cfg := DefaultFeedConfig("https://github.com/test/repo")
	out, err := RenderRSS(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error for empty releases: %v", err)
	}
	if !strings.Contains(out, "<rss") {
		t.Error("expected valid RSS even for empty releases")
	}
}

func TestRenderAtom_ValidXML(t *testing.T) {
	cfg := DefaultFeedConfig("https://github.com/test/repo")
	cfg.Title = "Test Atom Feed"

	out, err := RenderAtom(testReleases(), cfg)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}

	if !strings.HasPrefix(out, "<?xml") {
		t.Error("expected XML declaration at start")
	}
	if !strings.Contains(out, `xmlns="http://www.w3.org/2005/Atom"`) {
		t.Error("missing Atom namespace")
	}
	if !strings.Contains(out, "Test Atom Feed") {
		t.Error("missing feed title")
	}
	if !strings.Contains(out, "Release v2.0.0") {
		t.Error("missing entry for v2.0.0")
	}
	if !strings.Contains(out, "Release v1.2.0") {
		t.Error("missing entry for v1.2.0")
	}
}

func TestRenderAtom_Empty(t *testing.T) {
	cfg := DefaultFeedConfig("https://github.com/test/repo")
	out, err := RenderAtom(nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error for empty atom feed: %v", err)
	}
	if !strings.Contains(out, "<feed") {
		t.Error("expected <feed> element even for empty releases")
	}
}

func TestDefaultFeedConfig(t *testing.T) {
	cfg := DefaultFeedConfig("https://github.com/test/repo")
	if cfg.RepoURL != "https://github.com/test/repo" {
		t.Errorf("unexpected RepoURL: %s", cfg.RepoURL)
	}
	if cfg.Language != "en" {
		t.Errorf("expected language 'en', got %q", cfg.Language)
	}
	if cfg.FeedURL == "" {
		t.Error("expected non-empty FeedURL")
	}
}

func TestRenderRSS_ContainsChangelog(t *testing.T) {
	releases := []ReleaseNotes{{
		Version: "v1.0.0",
		Date:    time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Fixes:   []Entry{{Type: "fix", Description: "fix crash on startup"}},
	}}
	cfg := DefaultFeedConfig("https://github.com/test/repo")
	out, err := RenderRSS(releases, cfg)
	if err != nil {
		t.Fatalf("RenderRSS: %v", err)
	}
	if !strings.Contains(out, "fix crash on startup") {
		t.Error("expected fix description in RSS item")
	}
}
