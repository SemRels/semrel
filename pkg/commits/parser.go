// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package commits provides conventional commit parsing.
// See: https://github.com/GoSemantics/semrel/issues/3
package commits

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
type Parser struct {
// Compiled patterns for commit parsing
}

// NewParser creates a new conventional commit parser.
func NewParser() *Parser {
return &Parser{}
}

// Parse parses a commit message into a Commit struct.
// Follows the Conventional Commits specification: https://www.conventionalcommits.org/
// type(scope): description
// optional body
// optional BREAKING CHANGE: explanation
func (p *Parser) Parse(message string) (*Commit, error) {
// TODO: Implement parser
panic("not implemented")
}
