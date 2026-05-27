// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package releasenotes — RSS and Atom feed renderers.
// See: https://github.com/SemRels/semrel/issues/62
package releasenotes

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// FeedConfig holds the metadata used when generating RSS or Atom feeds.
type FeedConfig struct {
	Title       string // Feed title, e.g. "My Project Releases"
	Description string // Short description of the project
	RepoURL     string // Canonical repository URL, e.g. "https://github.com/org/repo"
	FeedURL     string // URL where this feed is hosted
	Language    string // BCP-47 language tag, e.g. "en" (RSS only)
}

// DefaultFeedConfig returns a minimal FeedConfig using the repository URL as base.
func DefaultFeedConfig(repoURL string) FeedConfig {
	return FeedConfig{
		Title:       "Releases",
		Description: "Project releases",
		RepoURL:     repoURL,
		FeedURL:     repoURL + "/releases.xml",
		Language:    "en",
	}
}

// --- RSS 2.0 structs --------------------------------------------------------

type rssRoot struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language,omitempty"`
	LastBuild   string    `xml:"lastBuildDate,omitempty"`
	AtomLink    atomLink  `xml:"http://www.w3.org/2005/Atom link"`
	Items       []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        guid   `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

type guid struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}

// RenderRSS generates an RSS 2.0 feed for a slice of release notes.
// Each ReleaseNotes becomes a feed item whose description is the Keep-a-Changelog
// formatted entry. releases should be ordered newest-first.
//
// See: https://github.com/SemRels/semrel/issues/62
func RenderRSS(releases []ReleaseNotes, cfg FeedConfig) (string, error) {
	items := make([]rssItem, 0, len(releases))
	for _, r := range releases {
		date := r.Date
		if date.IsZero() {
			date = time.Now().UTC()
		}
		releaseURL := fmt.Sprintf("%s/releases/tag/%s", cfg.RepoURL, r.Version)
		description := r.RenderKeepAChangelog(DefaultSectionConfig())
		items = append(items, rssItem{
			Title:       fmt.Sprintf("Release %s", r.Version),
			Link:        releaseURL,
			GUID:        guid{Value: releaseURL, IsPermaLink: "true"},
			PubDate:     date.UTC().Format(time.RFC1123Z),
			Description: description,
		})
	}

	feed := rssRoot{
		Version: "2.0",
		Channel: rssChannel{
			Title:       cfg.Title,
			Link:        cfg.RepoURL,
			Description: cfg.Description,
			Language:    cfg.Language,
			LastBuild:   time.Now().UTC().Format(time.RFC1123Z),
			AtomLink:    atomLink{Href: cfg.FeedURL, Rel: "self", Type: "application/rss+xml"},
			Items:       items,
		},
	}

	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling RSS feed: %w", err)
	}
	return xml.Header + string(out), nil
}

// --- Atom 1.0 structs -------------------------------------------------------

type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	XMLNS    string      `xml:"xmlns,attr"`
	Title    atomText    `xml:"title"`
	Subtitle atomText    `xml:"subtitle,omitempty"`
	ID       string      `xml:"id"`
	Updated  string      `xml:"updated"`
	Link     []atomFLink `xml:"link"`
	Entries  []atomEntry `xml:"entry"`
}

type atomText struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

type atomFLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomEntry struct {
	Title   atomText    `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    atomFLink   `xml:"link"`
	Summary atomContent `xml:"summary"`
}

type atomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// RenderAtom generates an Atom 1.0 feed for a slice of release notes.
// releases should be ordered newest-first.
//
// See: https://github.com/SemRels/semrel/issues/62
func RenderAtom(releases []ReleaseNotes, cfg FeedConfig) (string, error) {
	entries := make([]atomEntry, 0, len(releases))
	updated := time.Now().UTC()

	for _, r := range releases {
		date := r.Date
		if date.IsZero() {
			date = time.Now().UTC()
		}
		releaseURL := fmt.Sprintf("%s/releases/tag/%s", cfg.RepoURL, r.Version)
		description := r.RenderKeepAChangelog(DefaultSectionConfig())
		// Atom summary: plain text extract — first 500 chars of rendered changelog
		summary := strings.TrimSpace(description)
		if len(summary) > 500 {
			summary = summary[:497] + "..."
		}
		entries = append(entries, atomEntry{
			Title:   atomText{Value: fmt.Sprintf("Release %s", r.Version)},
			ID:      releaseURL,
			Updated: date.UTC().Format(time.RFC3339),
			Link:    atomFLink{Href: releaseURL, Rel: "alternate", Type: "text/html"},
			Summary: atomContent{Type: "text", Value: summary},
		})
	}

	if len(entries) > 0 {
		// Use the most recent entry date as the feed updated timestamp
		if releases[0].Date != (time.Time{}) {
			updated = releases[0].Date.UTC()
		}
	}

	feed := atomFeed{
		XMLNS:    "http://www.w3.org/2005/Atom",
		Title:    atomText{Value: cfg.Title},
		Subtitle: atomText{Value: cfg.Description},
		ID:       cfg.FeedURL,
		Updated:  updated.Format(time.RFC3339),
		Link: []atomFLink{
			{Href: cfg.FeedURL, Rel: "self", Type: "application/atom+xml"},
			{Href: cfg.RepoURL, Rel: "alternate", Type: "text/html"},
		},
		Entries: entries,
	}

	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling Atom feed: %w", err)
	}
	return xml.Header + string(out), nil
}
