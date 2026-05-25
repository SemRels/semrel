// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package commits provides conventional commit parsing.
// See: https://github.com/GoSemantics/semrel/issues/3
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
// Issue: https://github.com/GoSemantics/semrel/issues/3
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
