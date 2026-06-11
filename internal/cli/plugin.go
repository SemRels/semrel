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
  semrel plugin install <name>    — download and install a plugin`,
	}

	cmd.AddCommand(newPluginListCommand())
	cmd.AddCommand(newPluginSearchCommand())
	cmd.AddCommand(newPluginInstallCommand())
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

Config entries using category-prefixed names (e.g. "provider-github",
"condition-github-actions") are automatically resolved to their short registry
names ("github", "github-actions") and install the same binary.

Examples:
  semrel plugin install github
  semrel plugin install github@1.2.0
  semrel plugin install provider-github
  semrel plugin install condition-github-actions`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginInstall(cmd.Context(), args[0], pluginDir)
		},
	}
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "Override the plugin installation directory (default: .semrel/plugins/)")
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

	go func() {
		client, err := registry.NewRegistryClientFromEnv()
		if err != nil {
			return
		}
		client.TrackDownload(
			context.Background(),
			strings.TrimPrefix(meta.Namespace, "@"),
			meta.Name,
			versionEntry.Version,
			runtime.GOOS,
			runtime.GOARCH,
		)
	}()

	return nil
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
