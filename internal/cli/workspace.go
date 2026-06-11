// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/SemRels/semrel/pkg/config"
	"github.com/SemRels/semrel/pkg/monorepo"
)

func newWorkspaceCommand(configFile *string, outputFormat *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Orchestrate releases across a monorepo workspace",
		Long: `Manage releases for all packages in a monorepo workspace with a single command.

Configure the workspace in your root .semrel.yaml:

  workspace:
    strategy: independent   # each package gets its own version (default)
    packages:
      - path: packages/api
        tagPrefix: "api@v"
      - path: packages/ui
        tagPrefix: "ui@v"
        dependsOn: [packages/api]

Or use auto-discovery via a glob pattern:

  workspace:
    strategy: independent
    pattern: "packages/*"

Each package directory may have its own .semrel.yaml that extends or
overrides the root configuration (branches, rules, plugins, tagPrefix).

Subcommands:
  semrel workspace list              — list all workspace packages
  semrel workspace release           — release all packages (dependency order)
  semrel workspace release --dry-run — preview what would be released`,
	}
	cmd.AddCommand(newWorkspaceListCommand(configFile))
	cmd.AddCommand(newWorkspaceReleaseCommand(configFile, outputFormat))
	return cmd
}

// ── list ─────────────────────────────────────────────────────────────────────

func newWorkspaceListCommand(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all packages in the workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspaceList(cmd.Context(), *configFile)
		},
	}
}

func runWorkspaceList(ctx context.Context, configFile string) error {
	_, pkgs, err := resolveWorkspacePackages(ctx, configFile)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Println("no workspace packages found — check workspace.packages or workspace.pattern in .semrel.yaml")
		return nil
	}
	for _, p := range pkgs {
		deps := ""
		if len(p.DependsOn) > 0 {
			deps = fmt.Sprintf("  → depends on: %s", strings.Join(p.DependsOn, ", "))
		}
		fmt.Printf("  %-40s  tagPrefix=%s%s\n", p.Path, p.TagPrefix, deps)
	}
	return nil
}

// ── release ───────────────────────────────────────────────────────────────────

func newWorkspaceReleaseCommand(configFile *string, outputFormat *string) *cobra.Command {
	var dryRun bool
	var parallel bool
	var failFast bool
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release all workspace packages in dependency order",
		Long: `Release every package configured in the workspace.

Packages with no releasable commits since their last tag are skipped.
Packages are released in dependency order: packages listed in 'dependsOn'
are released before their dependents.

With --parallel, independent packages (no pending dependencies) are
released concurrently. The dependency contract is always honoured —
a package only starts when all its dependencies have succeeded.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspaceRelease(cmd.Context(), *configFile, *outputFormat, dryRun, parallel, failFast)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview releases without making any changes")
	cmd.Flags().BoolVar(&parallel, "parallel", false, "Release independent packages concurrently")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on the first package failure")
	return cmd
}

// workspaceResult holds the outcome of a single package release.
type workspaceResult struct {
	pkg     workspacePkg
	skipped bool
	err     error
}

// workspacePkg is an enriched package entry ready for execution.
type workspacePkg struct {
	monorepo.Package
	DependsOn []string
	ConfigFile string // resolved .semrel.yaml path for this package
}

func runWorkspaceRelease(ctx context.Context, rootConfigFile, outputFormat string, dryRun, parallel, failFast bool) error {
	rootCfg, pkgs, err := resolveWorkspacePackages(ctx, rootConfigFile)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Println("no workspace packages found — check workspace.packages or workspace.pattern in .semrel.yaml")
		return nil
	}

	if rootCfg.Workspace != nil && rootCfg.Workspace.FailFast {
		failFast = true
	}

	if parallel {
		return runWorkspaceParallel(ctx, pkgs, outputFormat, dryRun, failFast)
	}
	return runWorkspaceSequential(ctx, pkgs, outputFormat, dryRun, failFast)
}

// runWorkspaceSequential releases packages one at a time in topological order.
func runWorkspaceSequential(ctx context.Context, pkgs []workspacePkg, outputFormat string, dryRun, failFast bool) error {
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	defer func() { _ = os.Chdir(origDir) }() //nolint:errcheck

	ordered, err := topoSort(pkgs)
	if err != nil {
		return fmt.Errorf("resolving package dependency order: %w", err)
	}

	var errs []string
	for _, pkg := range ordered {
		label := fmt.Sprintf("[%s]", pkg.Path)

		if err := os.Chdir(pkg.Path); err != nil {
			msg := fmt.Sprintf("%s  error: cannot change to package directory: %v", label, err)
			if failFast {
				return fmt.Errorf("%s", msg)
			}
			errs = append(errs, msg)
			continue
		}

		fmt.Printf("%s  releasing…\n", label)
		releaseErr := runRelease(ctx, dryRun, pkg.ConfigFile, false, false, false, outputFormat, false, "", "")
		_ = os.Chdir(origDir) //nolint:errcheck

		switch {
		case releaseErr == nil:
			fmt.Printf("%s  ✓ released\n", label)
		case errors.Is(releaseErr, ErrNothingToRelease):
			fmt.Printf("%s  – skipped (nothing to release)\n", label)
		default:
			msg := fmt.Sprintf("%s  %v", label, releaseErr)
			if failFast {
				return fmt.Errorf("%s", msg)
			}
			errs = append(errs, msg)
			fmt.Fprintf(os.Stderr, "%s  ✗ failed: %v\n", label, releaseErr)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d package(s) failed:\n  - %s", len(errs), strings.Join(errs, "\n  - "))
	}
	return nil
}

// runWorkspaceParallel releases packages concurrently, respecting the dependency graph.
// Independent packages start immediately; a package waits until all its dependencies succeed.
func runWorkspaceParallel(ctx context.Context, pkgs []workspacePkg, outputFormat string, dryRun, failFast bool) error {
	type state struct {
		done chan struct{}
		err  error
	}

	byPath := make(map[string]*workspacePkg, len(pkgs))
	states := make(map[string]*state, len(pkgs))
	for i := range pkgs {
		byPath[pkgs[i].Path] = &pkgs[i]
		states[pkgs[i].Path] = &state{done: make(chan struct{})}
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    []string
		aborted bool
	)

	for i := range pkgs {
		pkg := pkgs[i]
		s := states[pkg.Path]
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer close(s.done)

			// Wait for all dependencies.
			for _, dep := range pkg.DependsOn {
				depState, ok := states[dep]
				if !ok {
					continue
				}
				select {
				case <-depState.done:
					if depState.err != nil {
						s.err = fmt.Errorf("dependency %q failed", dep)
						fmt.Fprintf(os.Stderr, "[%s]  skipped — dependency %q failed\n", pkg.Path, dep)
						return
					}
				case <-ctx.Done():
					s.err = ctx.Err()
					return
				}
			}

			mu.Lock()
			if aborted {
				mu.Unlock()
				s.err = fmt.Errorf("aborted due to previous failure")
				return
			}
			mu.Unlock()

			label := fmt.Sprintf("[%s]", pkg.Path)
			fmt.Printf("%s  releasing…\n", label)

			// Each goroutine needs its own working directory — use subprocess via os.Executable.
			releaseErr := runWorkspacePackageInSubprocess(ctx, pkg, outputFormat, dryRun)

			if releaseErr != nil {
				s.err = releaseErr
				fmt.Fprintf(os.Stderr, "%s  ✗ failed: %v\n", label, releaseErr)
				mu.Lock()
				errs = append(errs, fmt.Sprintf("[%s]: %v", pkg.Path, releaseErr))
				if failFast {
					aborted = true
				}
				mu.Unlock()
			} else {
				fmt.Printf("%s  ✓ done\n", label)
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%d package(s) failed:\n  - %s", len(errs), strings.Join(errs, "\n  - "))
	}
	return nil
}

// runWorkspacePackageInSubprocess launches the current semrel binary as a subprocess
// with the package directory as the working directory.
func runWorkspacePackageInSubprocess(ctx context.Context, pkg workspacePkg, outputFormat string, dryRun bool) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving semrel executable: %w", err)
	}

	args := []string{"release", "--output", outputFormat}
	if dryRun {
		args = append(args, "--dry-run")
	}
	if pkg.ConfigFile != "" {
		args = append(args, "--config", pkg.ConfigFile)
	}

	cmd := exec.CommandContext(ctx, self, args...)
	cmd.Dir = pkg.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveWorkspacePackages loads the root config and resolves the package list.
func resolveWorkspacePackages(_ context.Context, rootConfigFile string) (*config.Config, []workspacePkg, error) {
	cfgFile := rootConfigFile
	if cfgFile == "" {
		found, err := config.FindConfigFile(".")
		if err != nil {
			return nil, nil, fmt.Errorf("no config file found: %w", err)
		}
		cfgFile = found
	}

	rootCfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	if rootCfg.Workspace == nil {
		return rootCfg, nil, fmt.Errorf("no workspace configuration found in %s — add a 'workspace:' section", cfgFile)
	}

	ws := rootCfg.Workspace
	rootDir := filepath.Dir(cfgFile)

	var pkgs []workspacePkg

	switch {
	case len(ws.Packages) > 0:
		// Explicit list.
		for _, ref := range ws.Packages {
			pkgPath := filepath.Join(rootDir, ref.Path)
			tagPrefix := ref.TagPrefix
			if tagPrefix == "" {
				tagPrefix = ref.Path + "@v"
			}
			pkgs = append(pkgs, workspacePkg{
				Package: monorepo.Package{
					Name:      ref.Path,
					Path:      pkgPath,
					TagPrefix: tagPrefix,
				},
				DependsOn:  resolveDepPaths(ref.DependsOn, rootDir),
				ConfigFile: resolvePackageConfig(pkgPath, rootCfg),
			})
		}

	case ws.Pattern != "":
		// Auto-discover via glob pattern.
		d := monorepo.NewDiscoverer(rootDir)
		discovered, err := d.DiscoverFromPatterns([]string{ws.Pattern})
		if err != nil {
			return rootCfg, nil, fmt.Errorf("discovering packages: %w", err)
		}
		for _, mp := range discovered {
			pkgs = append(pkgs, workspacePkg{
				Package:    mp,
				ConfigFile: resolvePackageConfig(mp.Path, rootCfg),
			})
		}

	default:
		return rootCfg, nil, fmt.Errorf("workspace requires either 'packages' list or 'pattern' in .semrel.yaml")
	}

	return rootCfg, pkgs, nil
}

// resolveDepPaths converts relative dependency paths to absolute.
func resolveDepPaths(deps []string, rootDir string) []string {
	abs := make([]string, len(deps))
	for i, d := range deps {
		abs[i] = filepath.Join(rootDir, d)
	}
	return abs
}

// resolvePackageConfig returns the config file for a package directory.
// If the package has its own .semrel.yaml, that is used (it inherits from root
// via the package's own LoadConfig). Otherwise the root config path is returned.
func resolvePackageConfig(pkgPath string, rootCfg *config.Config) string {
	for _, name := range []string{".semrel.yaml", ".semrel.yml", ".semrel.toml", ".semrel.json"} {
		candidate := filepath.Join(pkgPath, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// No package-specific config — return empty string so runRelease auto-detects.
	// The release will look for the config starting from pkgPath (after Chdir).
	return ""
}

// topoSort returns a topological ordering of packages respecting DependsOn edges.
// Returns an error if a cycle is detected.
func topoSort(pkgs []workspacePkg) ([]workspacePkg, error) {
	byPath := make(map[string]*workspacePkg, len(pkgs))
	for i := range pkgs {
		byPath[pkgs[i].Path] = &pkgs[i]
	}

	var (
		result  []workspacePkg
		visited = make(map[string]bool)
		inStack = make(map[string]bool)
	)

	var visit func(path string) error
	visit = func(path string) error {
		if inStack[path] {
			return fmt.Errorf("dependency cycle detected at %q", path)
		}
		if visited[path] {
			return nil
		}
		inStack[path] = true
		pkg, ok := byPath[path]
		if !ok {
			return fmt.Errorf("package %q not found in workspace", path)
		}
		for _, dep := range pkg.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		inStack[path] = false
		visited[path] = true
		result = append(result, *pkg)
		return nil
	}

	for _, pkg := range pkgs {
		if err := visit(pkg.Path); err != nil {
			return nil, err
		}
	}
	return result, nil
}

