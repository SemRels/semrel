// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package git provides Git repository operations.
// See: https://github.com/GoSemantics/semrel/issues/5
package git

import "context"

// Repository represents a Git repository.
type Repository struct {
path string
}

// OpenRepository opens a Git repository at the given path.
// Issue: https://github.com/GoSemantics/semrel/issues/5
func OpenRepository(path string) (*Repository, error) {
// TODO: Implement git.Open
panic("not implemented")
}

// LastTag returns the most recent Git tag.
func (r *Repository) LastTag(ctx context.Context) (string, error) {
// TODO: Implement
panic("not implemented")
}

// CommitsSince returns commits since the given ref.
func (r *Repository) CommitsSince(ctx context.Context, ref string) ([]string, error) {
// TODO: Implement
panic("not implemented")
}
