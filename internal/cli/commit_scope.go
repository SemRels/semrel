// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"path/filepath"
)

type commitPathsKey struct{}

// withCommitPaths returns a context that restricts the release commit range to
// commits touching at least one of the given paths (monorepo packages).
// Relative paths are resolved against the current working directory now, so the
// scope stays valid after the workspace runner changes directory.
func withCommitPaths(ctx context.Context, paths ...string) context.Context {
	abs := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if a, err := filepath.Abs(p); err == nil {
			p = a
		}
		abs = append(abs, p)
	}
	if len(abs) == 0 {
		return ctx
	}
	return context.WithValue(ctx, commitPathsKey{}, abs)
}

// commitPathsFrom returns the commit path scope stored in ctx, if any.
func commitPathsFrom(ctx context.Context) []string {
	paths, _ := ctx.Value(commitPathsKey{}).([]string)
	return paths
}
