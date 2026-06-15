// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/SemRels/semrel/internal/registry"
	"github.com/SemRels/semrel/pkg/plugin"
)

func newPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage semrel plugins",
		Long: `Commands to discover, install and manage semrel plugins.

Plugins extend semrel with providers, conditions, hooks and updaters.
They are fetched from the plugin registry at https://semrels.github.io/semrel-registry.

Plugin phases (configured per plugin in .semrel.yaml):
  condition — runs before commit analysis; failure aborts the release
  pre-tag   — runs after changelog generation, before the git tag is created
  release   — runs after tagging (providers, hooks, package publishers) [default]

Subcommands:
  semrel plugin list              — list all available plugins
  semrel plugin list --sort downloads
                               — list plugins by popularity
  semrel plugin search <query>    — search plugins by name or description
  semrel plugin install <name>    — download, install and lock a plugin
  semrel plugin update            — check or update plugins from .semrel.lock
  semrel plugin restore           — install all plugins from .semrel.lock`,
	}

	cmd.AddCommand(newPluginListCommand())
	cmd.AddCommand(newPluginSearchCommand())
	cmd.AddCommand(newPluginInstallCommand())
	cmd.AddCommand(newPluginUpdateCommand())
	cmd.AddCommand(newPluginRestoreCommand())
	return cmd
}

func newPluginListCommand() *cobra.Command {
	var noHeader bool
	var sortBy string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available plugins in the registry",
		Long: `List all available plugins in the registry.

Sort options:
  name       — alphabetical (default)
  downloads  — most downloaded first`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginList(cmd.Context(), noHeader, sortBy)
		},
	}
	cmd.Flags().BoolVar(&noHeader, "no-header", false, "Suppress the table header")
	cmd.Flags().StringVar(&sortBy, "sort", "name", "Sort by: name, downloads")
	return cmd
}

func newPluginSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search plugins by name, description or tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginSearch(cmd.Context(), args[0])
		},
	}
	return cmd
}

func newPluginInstallCommand() *cobra.Command {
	var pluginDir string
	cmd := &cobra.Command{
		Use:   "install <name[@version]>",
		Short: "Download and install a plugin binary",
		Long: `Download a plugin binary from the registry and install it into .semrel/plugins/.

The plugin binary is placed in .semrel/plugins/ relative to the current working
directory so that it is local to the project. Use --plugin-dir to override the
installation path (e.g. ~/.semrel/plugins/ for a user-global install).

When a plugin belongs to a namespace the full reference is required:

  semrel plugin install @semrel/github
  semrel plugin install @semrel/github@1.2.0

Bare names (without "@namespace/") are only accepted for plugins that have no
namespace in the registry. Config entries using category-prefixed names
(e.g. "provider-github", "condition-github-actions") are resolved to their short
registry names and follow the same namespace rule.

Examples:
  semrel plugin install @semrel/github
  semrel plugin install @semrel/github@1.2.0
  semrel plugin install github          (only if registry entry has no namespace)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginInstall(cmd.Context(), args[0], pluginDir)
		},
	}
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "Override the plugin installation directory (default: .semrel/plugins/)")
	return cmd
}

func newPluginUpdateCommand() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update [name[@version]]",
		Short: "Check or update installed plugins",
		Long: `Check for newer plugin versions in the registry and install updates.

Without arguments, semrel uses .semrel.lock as the source of truth and checks
or updates every pinned plugin.

With a plugin argument, semrel checks or updates only that plugin. A version can
be pinned explicitly (for example @semrel/github@1.2.0).

Examples:
  semrel plugin update
  semrel plugin update --check
  semrel plugin update @semrel/github
  semrel plugin update @semrel/github@1.2.0`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			return runPluginUpdate(cmd.Context(), target, checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, do not install")
	return cmd
}

func runPluginList(ctx context.Context, noHeader bool, sortBy string) error {
	reg, err := fetchRegistry(ctx)
	if err != nil {
		return err
	}

	plugins := append([]registry.PluginMeta(nil), reg.Plugins...)
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "downloads":
		sort.Slice(plugins, func(i, j int) bool {
			if plugins[i].Downloads != plugins[j].Downloads {
				return plugins[i].Downloads > plugins[j].Downloads
			}
			return pluginRef(plugins[i]) < pluginRef(plugins[j])
		})
	default:
		sort.Slice(plugins, func(i, j int) bool {
			return pluginRef(plugins[i]) < pluginRef(plugins[j])
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if !noHeader {
		_, _ = fmt.Fprintln(w, "NAME\tCATEGORY\tDOWNLOADS\tDESCRIPTION")
		_, _ = fmt.Fprintln(w, "----\t--------\t---------\t-----------")
	}
	for _, p := range plugins {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", pluginRef(p), p.Category, p.Downloads, truncate(p.Description, 50))
	}
	return w.Flush()
}

func runPluginSearch(ctx context.Context, query string) error {
	reg, err := fetchRegistry(ctx)
	if err != nil {
		return err
	}

	q := strings.ToLower(query)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION")
	_, _ = fmt.Fprintln(w, "----\t--------\t-----------")

	found := 0
	for _, p := range reg.Plugins {
		if strings.Contains(strings.ToLower(pluginRef(p)), q) ||
			strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Description), q) ||
			strings.Contains(strings.ToLower(p.Category), q) ||
			containsTag(p.Tags, q) {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", pluginRef(p), p.Category, truncate(p.Description, 60))
			found++
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if found == 0 {
		fmt.Fprintf(os.Stderr, "no plugins matching %q\n", query)
	}
	return nil
}

func runPluginInstall(ctx context.Context, nameVer, overrideDir string) error {
	name, ver := splitNameVersion(nameVer)

	loader := plugin.NewLoader()
	reg, err := loader.FetchMetadata(ctx)
	if err != nil {
		return fmt.Errorf("fetching registry: %w", err)
	}

	meta, err := reg.FindPlugin(name)
	if err != nil {
		return fmt.Errorf("plugin %q not found in registry", name)
	}

	if err := requireNamespacedReference(name, meta, "install"); err != nil {
		return err
	}

	versionEntry, err := meta.FindVersion(ver)
	if err != nil {
		return fmt.Errorf("version %q not available for plugin %q", ver, name)
	}

	if len(versionEntry.Checksums) == 0 {
		return fmt.Errorf("plugin %q@%s has no binary releases yet; check back later", name, versionEntry.Version)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Installing %s@%s ...\n", pluginRef(*meta), versionEntry.Version)

	// Use meta.Name (canonical registry name) for the download so that the cache key
	// is stable regardless of whether the user specified a category-prefixed name.
	binaryPath, err := loader.ResolvePluginBinary(ctx, meta.Name, versionEntry.Version)
	if err != nil {
		return fmt.Errorf("installing plugin: %w", err)
	}

	installDir := overrideDir
	if installDir == "" {
		installDir = filepath.Join(".semrel", "plugins")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}

	// Derive the binary name from the canonical registry name so that config entries
	// using either the short name ("github") or a category-prefixed name ("provider-github")
	// resolve to the same binary on disk ("semrel-plugin-github").
	binaryName := pluginBinaryName(meta.Name)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	dest := filepath.Join(installDir, binaryName)

	if err := copyFile(binaryPath, dest, 0o755); err != nil {
		return fmt.Errorf("installing plugin binary: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✓ Installed %s to %s\n", pluginRef(*meta), dest)

	// Update .semrel.lock so the installed version is pinned for all contributors.
	// Only update the lock when installing to the default project-local directory.
	if overrideDir == "" {
		if lockErr := updateLockFile(meta, versionEntry); lockErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: could not update %s: %v\n", LockFileName, lockErr)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "  updated %s\n", LockFileName)
		}
	}

	go func() {
		client, err := registry.NewRegistryClientFromEnv()
		if err != nil {
			return
		}
		client.TrackDownload(
			ctx,
			strings.TrimPrefix(meta.Namespace, "@"),
			meta.Name,
			versionEntry.Version,
			runtime.GOOS,
			runtime.GOARCH,
		)
	}()

	return nil
}

type pluginUpdateTarget struct {
	Ref             string
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
}

func runPluginUpdate(ctx context.Context, nameVer string, checkOnly bool) error {
	loader := plugin.NewLoader()
	reg, err := loader.FetchMetadata(ctx)
	if err != nil {
		return fmt.Errorf("fetching registry: %w", err)
	}

	targets, err := pluginUpdateTargets(reg, nameVer)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Printf("nothing to update — %s has no plugin entries\n", LockFileName)
		return nil
	}

	for _, target := range targets {
		switch {
		case target.CurrentVersion == "":
			fmt.Printf("• %s: not installed, latest is %s\n", target.Ref, target.LatestVersion)
		case target.UpdateAvailable:
			fmt.Printf("• %s: %s → %s\n", target.Ref, target.CurrentVersion, target.LatestVersion)
		default:
			fmt.Printf("• %s: up to date (%s)\n", target.Ref, target.CurrentVersion)
		}
	}

	if checkOnly {
		return nil
	}

	updated := 0
	for _, target := range targets {
		if !target.UpdateAvailable && target.CurrentVersion != "" {
			continue
		}
		if err := runPluginInstall(ctx, target.Ref+"@"+target.LatestVersion, ""); err != nil {
			return err
		}
		updated++
	}

	if updated == 0 {
		fmt.Println("all plugins are already up to date")
		return nil
	}
	fmt.Printf("updated %d plugin(s)\n", updated)
	return nil
}

func pluginUpdateTargets(reg *registry.PluginRegistry, nameVer string) ([]pluginUpdateTarget, error) {
	if strings.TrimSpace(nameVer) != "" {
		name, requestedVersion := splitNameVersion(nameVer)
		meta, err := reg.FindPlugin(name)
		if err != nil {
			return nil, fmt.Errorf("plugin %q not found in registry", name)
		}
		if err := requireNamespacedReference(name, meta, "update"); err != nil {
			return nil, err
		}

		var versionEntry *registry.PluginVersion
		if requestedVersion != "" {
			versionEntry, err = meta.FindVersion(requestedVersion)
			if err != nil {
				return nil, fmt.Errorf("version %q not available for plugin %q", requestedVersion, name)
			}
		} else {
			versionEntry, err = latestStableVersion(meta)
			if err != nil {
				return nil, err
			}
		}

		lf, err := ReadLockFile()
		if err != nil {
			return nil, err
		}
		ref := pluginRef(*meta)
		current := ""
		for _, binaryName := range pluginBinaryNames(meta.Name) {
			if entry := lf.FindByBinaryName(binaryName); entry != nil {
				current = entry.Version
				if entry.Ref != "" {
					ref = entry.Ref
				}
				break
			}
		}

		return []pluginUpdateTarget{{
			Ref:             ref,
			CurrentVersion:  current,
			LatestVersion:   versionEntry.Version,
			UpdateAvailable: current == "" || isVersionNewer(current, versionEntry.Version),
		}}, nil
	}

	lf, err := ReadLockFile()
	if err != nil {
		return nil, err
	}
	out := make([]pluginUpdateTarget, 0, len(lf.Plugins))
	for _, entry := range lf.Plugins {
		meta, err := reg.FindPlugin(entry.Ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s not found in registry; skipping\n", entry.Ref)
			continue
		}
		latest, err := latestStableVersion(meta)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: latest version lookup failed for %s: %v\n", entry.Ref, err)
			continue
		}
		out = append(out, pluginUpdateTarget{
			Ref:             entry.Ref,
			CurrentVersion:  entry.Version,
			LatestVersion:   latest.Version,
			UpdateAvailable: isVersionNewer(entry.Version, latest.Version),
		})
	}
	return out, nil
}

func isVersionNewer(current, candidate string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "v")
	if current == "" {
		return true
	}

	var (
		cMaj, cMin, cPatch int
		nMaj, nMin, nPatch int
	)
	if _, err := fmt.Sscanf(current, "%d.%d.%d", &cMaj, &cMin, &cPatch); err != nil {
		return candidate != current
	}
	if _, err := fmt.Sscanf(candidate, "%d.%d.%d", &nMaj, &nMin, &nPatch); err != nil {
		return candidate != current
	}
	if nMaj != cMaj {
		return nMaj > cMaj
	}
	if nMin != cMin {
		return nMin > cMin
	}
	return nPatch > cPatch
}

func latestStableVersion(meta *registry.PluginMeta) (*registry.PluginVersion, error) {
	var best *registry.PluginVersion
	for i := range meta.Versions {
		version := &meta.Versions[i]
		if version.Prerelease {
			continue
		}
		if best == nil || isVersionNewer(best.Version, version.Version) {
			best = version
		}
	}
	if best != nil {
		return best, nil
	}

	for i := range meta.Versions {
		version := &meta.Versions[i]
		if best == nil || isVersionNewer(best.Version, version.Version) {
			best = version
		}
	}
	if best == nil {
		return nil, fmt.Errorf("plugin %q has no available versions", meta.Name)
	}
	return best, nil
}

func requireNamespacedReference(name string, meta *registry.PluginMeta, action string) error {
	// Namespace enforcement: if the matched plugin belongs to a namespace but the
	// user did not supply one, require the full "@namespace/name" reference.
	// A bare name (e.g. "github") is only accepted for namespace-less plugins.
	if strings.HasPrefix(name, "@") || meta.Namespace == "" {
		return nil
	}
	ns := meta.Namespace
	if !strings.HasPrefix(ns, "@") {
		ns = "@" + ns
	}
	return fmt.Errorf(
		"plugin %q belongs to namespace %s — use the full reference:\n  semrel plugin %s %s/%s",
		meta.Name, ns, action, ns, meta.Name,
	)
}

// updateLockFile upserts the entry for meta/version into .semrel.lock.
func updateLockFile(meta *registry.PluginMeta, version *registry.PluginVersion) error {
	lf, err := ReadLockFile()
	if err != nil {
		return err
	}
	lf.Upsert(PluginLockEntry{
		BinaryName: pluginBinaryName(meta.Name),
		Ref:        pluginRef(*meta),
		Version:    version.Version,
		Checksums:  version.Checksums,
	})
	return lf.Write()
}

func fetchRegistry(ctx context.Context) (*registry.PluginRegistry, error) {
	client, err := registry.NewRegistryClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("initialising registry client: %w", err)
	}
	reg, err := client.FetchMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching registry: %w", err)
	}
	return reg, nil
}

func splitNameVersion(s string) (name, version string) {
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func containsTag(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func copyFile(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, perm)
}

// pluginRef returns the canonical display reference for a plugin:
// "@namespace/name" when a namespace is present, otherwise just "name".
func pluginRef(p registry.PluginMeta) string {
	if p.Namespace != "" {
		ns := p.Namespace
		if !strings.HasPrefix(ns, "@") {
			ns = "@" + ns
		}
		return ns + "/" + p.Name
	}
	return p.Name
}

func newPluginRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Install all plugins from .semrel.lock",
		Long: `Download and install every plugin pinned in .semrel.lock.

Use this on CI or after cloning a repository to ensure you have exactly the
same plugin versions as recorded in the lock file.

The lock file (.semrel.lock) is created and updated automatically by
'semrel plugin install'. Commit it alongside .semrel.yaml.

Example CI step:
  - run: semrel plugin restore`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginRestore(cmd.Context())
		},
	}
	return cmd
}

func runPluginRestore(ctx context.Context) error {
	lf, err := ReadLockFile()
	if err != nil {
		return err
	}
	if len(lf.Plugins) == 0 {
		fmt.Printf("nothing to restore — %s has no plugin entries\n", LockFileName)
		return nil
	}

	installDir := filepath.Join(".semrel", "plugins")
	loader := plugin.NewLoader()

	var failed []string
	for _, entry := range lf.Plugins {
		binaryName := entry.BinaryName
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		dest := filepath.Join(installDir, binaryName)

		// Skip if the binary is already present AND the checksum matches.
		// If the file exists but the checksum differs (e.g. an old version from
		// a stale CI cache), re-download so the lock file version is enforced.
		if info, statErr := os.Stat(dest); statErr == nil && !info.IsDir() {
			platform := runtime.GOOS + "_" + runtime.GOARCH
			if expected, ok := entry.Checksums[platform]; ok {
				if chkErr := loader.ValidateChecksum(dest, expected); chkErr == nil {
					fmt.Printf("✓  %s@%s already installed\n", entry.Ref, entry.Version)
					continue
				}
				// Checksum mismatch — fall through to re-download.
				fmt.Printf("↻  %s binary present but checksum differs — updating to @%s\n", entry.Ref, entry.Version)
			} else {
				// No checksum for this platform — trust the existing file.
				fmt.Printf("✓  %s@%s already installed\n", entry.Ref, entry.Version)
				continue
			}
		}

		fmt.Printf("⬇  restoring %s@%s ...\n", entry.Ref, entry.Version)

		// Extract the bare plugin name for the registry/cache lookup.
		bareName := entry.Ref
		if idx := strings.LastIndex(bareName, "/"); idx >= 0 {
			bareName = bareName[idx+1:]
		}

		binaryPath, dlErr := loader.ResolvePluginBinary(ctx, bareName, entry.Version)
		if dlErr != nil {
			fmt.Fprintf(os.Stderr, "✗  failed to restore %s: %v\n", entry.Ref, dlErr)
			failed = append(failed, entry.Ref)
			continue
		}

		// Verify the downloaded binary against the checksum stored in the lock file.
		// (ResolvePluginBinary may return a cached copy; re-verify here for safety.)
		platform := runtime.GOOS + "_" + runtime.GOARCH
		if expected, ok := entry.Checksums[platform]; ok {
			if chkErr := loader.ValidateChecksum(binaryPath, expected); chkErr != nil {
				fmt.Fprintf(os.Stderr, "✗  checksum mismatch for %s: %v\n", entry.Ref, chkErr)
				failed = append(failed, entry.Ref)
				continue
			}
		}

		if err := os.MkdirAll(installDir, 0o755); err != nil {
			return fmt.Errorf("creating plugin directory: %w", err)
		}
		if err := copyFile(binaryPath, dest, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "✗  failed to install %s: %v\n", entry.Ref, err)
			failed = append(failed, entry.Ref)
			continue
		}
		fmt.Printf("✓  restored %s@%s → %s\n", entry.Ref, entry.Version, dest)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to restore %d plugin(s): %v", len(failed), failed)
	}
	return nil
}
