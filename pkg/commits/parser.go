// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package commits provides conventional commit parsing.
// See: https://github.com/SemRels/semrel/issues/3
package commits

import (
	"fmt"
	"strings"
)

// Commit represents a parsed conventional commit.
type Commit struct {
	Type             string
	Scope            string
	Description      string
	Body             string
	IsBreakingChange bool
	Raw              string
}

// Parser parses conventional commits.
// Issue: https://github.com/SemRels/semrel/issues/3
type Parser struct{}

// NewParser creates a new conventional commit parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a commit message into a Commit struct.
// Follows the Conventional Commits specification: https://www.conventionalcommits.org/
//
// Supported formats:
//
//	type(scope): description
//	type!: description         (breaking change)
//	type(scope)!: description  (scoped breaking change)
//
// Body lines with "BREAKING CHANGE: ..." also mark the commit as breaking.
func (p *Parser) Parse(message string) (*Commit, error) {
	if message == "" {
		return nil, fmt.Errorf("empty commit message")
	}

	lines := strings.SplitN(message, "\n", 2)
	subject := strings.TrimSpace(lines[0])
	body := ""
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
	}

	// Parse subject: type(scope)!: description
	colonIdx := strings.Index(subject, ": ")
	if colonIdx < 0 {
		// Not a conventional commit — treat as untyped
		return &Commit{Raw: message, Description: subject}, nil
	}

	prefix := subject[:colonIdx]
	description := strings.TrimSpace(subject[colonIdx+2:])

	isBreaking := strings.HasSuffix(prefix, "!")
	if isBreaking {
		prefix = prefix[:len(prefix)-1]
	}

	commitType := prefix
	scope := ""
	if openParen := strings.Index(prefix, "("); openParen >= 0 {
		closeParen := strings.Index(prefix, ")")
		if closeParen > openParen {
			commitType = prefix[:openParen]
			scope = prefix[openParen+1 : closeParen]
		}
	}

	// Check for BREAKING CHANGE in body
	if !isBreaking && strings.Contains(body, "BREAKING CHANGE:") {
		isBreaking = true
	}

	return &Commit{
		Type:             commitType,
		Scope:            scope,
		Description:      description,
		Body:             body,
		IsBreakingChange: isBreaking,
		Raw:              message,
	}, nil
}

// ParseAll parses multiple raw commit messages, skipping unparseable ones.
func (p *Parser) ParseAll(messages []string) []*Commit {
	result := make([]*Commit, 0, len(messages))
	for _, msg := range messages {
		c, err := p.Parse(msg)
		if err == nil && c != nil {
			result = append(result, c)
		}
	}
	return result
}

// ParseMulti parses a commit message that may contain multiple conventional commit
// entries — for example, a squash-merge message with multiple "type: desc" lines.
// It returns one Commit per recognized conventional commit line found in the message.
// The highest-priority type (breaking > feat > fix/perf/revert > other) is used for
// the first returned commit so callers that only look at [0] still get the right bump.
// See: https://github.com/SemRels/semrel/issues/107
func (p *Parser) ParseMulti(message string) ([]*Commit, error) {
	if message == "" {
		return nil, fmt.Errorf("empty commit message")
	}

	var result []*Commit
	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		c, err := p.Parse(line)
		if err == nil && c != nil && c.Type != "" {
			result = append(result, c)
		}
	}

	// If no multi-type entries found, fall back to whole-message parse
	if len(result) == 0 {
		c, err := p.Parse(message)
		if err != nil {
			return nil, err
		}
		return []*Commit{c}, nil
	}

	// Sort by bump priority so the highest-impact commit is first
	result = sortByPriority(result)
	return result, nil
}

// bumpPriority returns a numeric priority for sorting (higher = more impactful).
func bumpPriority(c *Commit) int {
	if c.IsBreakingChange {
		return 4
	}
	switch c.Type {
	case "feat":
		return 3
	case "fix", "perf", "revert":
		return 2
	default:
		return 1
	}
}

func sortByPriority(cs []*Commit) []*Commit {
	// Insertion sort (typically few items — no need for sort.Slice overhead)
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && bumpPriority(cs[j]) > bumpPriority(cs[j-1]); j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
	return cs
}
