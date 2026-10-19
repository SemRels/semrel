// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"path/filepath"

	gitpkg "github.com/SemRels/semrel/pkg/git"
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

// commitScope restricts a release's commit range to the commits that belong to
// one workspace package.
type commitScope struct {
	// PackagePath is the absolute package root.
	PackagePath string
	// SharedPaths are repo-root-relative globs; IncludeShared decides whether
	// commits touching them count for this package.
	SharedPaths   []string
	IncludeShared bool
	// IgnorePaths are package-relative globs that never release the package.
	IgnorePaths []string
}

type commitScopeKey struct{}

// withCommitScope returns ctx restricted to the given package scope. The
// package path is made absolute now so it survives the runner's chdir.
func withCommitScope(ctx context.Context, sc commitScope) context.Context {
	if abs, err := filepath.Abs(sc.PackagePath); err == nil {
		sc.PackagePath = abs
	}
	return context.WithValue(ctx, commitScopeKey{}, &sc)
}

func commitScopeFrom(ctx context.Context) *commitScope {
	sc, _ := ctx.Value(commitScopeKey{}).(*commitScope)
	return sc
}

// commitsForRelease lists the commits since ref, restricted to the package
// scope or path list in ctx when present.
func commitsForRelease(ctx context.Context, repo *gitpkg.Repository, ref string) ([]gitpkg.Commit, error) {
	if sc := commitScopeFrom(ctx); sc != nil {
		specs, err := repo.PackagePathspecs(sc.PackagePath, sc.SharedPaths, sc.IncludeShared, sc.IgnorePaths)
		if err != nil {
			return nil, err
		}
		return repo.CommitsDetailedSincePathspecs(ctx, ref, specs)
	}
	return repo.CommitsDetailedSince(ctx, ref, commitPathsFrom(ctx)...)
}
