// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package issues provides issue tracker integration for semrel.
// When commits reference issue tracker tickets (Jira, Linear, GitHub Issues,
// GitLab Issues), semrel can automatically close or transition those issues
// after a successful release.
//
// Supported reference patterns:
//   - Jira: PROJECT-123 (e.g. "feat(auth): add OAuth2 closes PROJ-456")
//   - Linear: linear-123 or LIN-123, and Linear team-prefixed IDs (e.g. ENG-123)
//   - GitHub: #123 or GH-123
//   - Generic: closes #123, fixes #123, resolves #123
//
// See: https://github.com/SemRels/semrel/issues/49
package issues

import (
	"regexp"
	"strings"
)

// IssueRef represents a single issue reference extracted from a commit message.
type IssueRef struct {
	// Tracker identifies the issue tracking system.
	Tracker string
	// ID is the tracker-specific issue identifier (e.g. "PROJ-456", "123").
	ID string
	// Raw is the original matched text.
	Raw string
}

// Extractor extracts issue references from commit messages.
type Extractor struct {
	patterns []*issuePattern
}

type issuePattern struct {
	tracker string
	re      *regexp.Regexp
	// group is the capture group index that contains the issue ID.
	group int
}

// DefaultExtractor returns an Extractor with all built-in patterns.
func DefaultExtractor() *Extractor {
	return &Extractor{
		patterns: []*issuePattern{
			// Jira: PROJECT-123 (project key is 2-10 uppercase letters)
			{tracker: "jira", re: regexp.MustCompile(`\b([A-Z]{2,10}-\d+)\b`), group: 1},
			// GitHub: closes/fixes/resolves #123 or just #123
			{tracker: "github", re: regexp.MustCompile(`(?i)(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)?\s*#(\d+)`), group: 1},
		},
	}
}

// Extract returns all unique issue references found in the commit message.
// References are deduplicated by Tracker+ID.
func (e *Extractor) Extract(message string) []IssueRef {
	seen := make(map[string]bool)
	var refs []IssueRef

	for _, p := range e.patterns {
		matches := p.re.FindAllStringSubmatch(message, -1)
		for _, m := range matches {
			if len(m) <= p.group {
				continue
			}
			id := m[p.group]
			key := p.tracker + ":" + id
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, IssueRef{
				Tracker: p.tracker,
				ID:      id,
				Raw:     m[0],
			})
		}
	}
	return refs
}

// ExtractAll extracts issue references from a list of commit messages and
// returns all unique references across all messages.
func (e *Extractor) ExtractAll(messages []string) []IssueRef {
	seen := make(map[string]bool)
	var refs []IssueRef

	for _, msg := range messages {
		for _, ref := range e.Extract(msg) {
			key := ref.Tracker + ":" + ref.ID
			if !seen[key] {
				seen[key] = true
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

// Filter returns only the refs matching the given tracker name.
func Filter(refs []IssueRef, tracker string) []IssueRef {
	var filtered []IssueRef
	for _, r := range refs {
		if strings.EqualFold(r.Tracker, tracker) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
