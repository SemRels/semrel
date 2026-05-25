// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/GoSemantics/semrel/internal/registry"
	"github.com/GoSemantics/semrel/pkg/plugin"
	"github.com/spf13/cobra"
)

func newPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage semrel plugins",
		Long: `Commands to discover, install and manage semrel plugins.

Plugins extend semrel with additional providers, analyzers, conditions,
hooks and updaters. They are fetched from the plugin registry at
https://semrels.github.io/semrel-registry.`,
	}

	cmd.AddCommand(newPluginListCommand())
	cmd.AddCommand(newPluginSearchCommand())
	cmd.AddCommand(newPluginInstallCommand())
	return cmd
}

func newPluginListCommand() *cobra.Command {
	var noHeader bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available plugins in the registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginList(cmd.Context(), noHeader)
		},
	}
	cmd.Flags().BoolVar(&noHeader, "no-header", false, "Suppress the table header")
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
		Long: `Download a plugin binary from the registry and install it into ~/.semrel/plugins/.

Examples:
  semrel plugin install npm
  semrel plugin install npm@1.2.0`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginInstall(cmd.Context(), args[0], pluginDir)
		},
	}
	cmd.Flags().StringVar(&pluginDir, "plugin-dir", "", "Override the plugin installation directory (default: ~/.semrel/plugins/)")
	return cmd
}

func runPluginList(ctx context.Context, noHeader bool) error {
	reg, err := fetchRegistry(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION\tVERSIONS")
		fmt.Fprintln(w, "----\t--------\t-----------\t--------")
	}
	for _, p := range reg.Plugins {
		latest := "-"
		if len(p.Versions) > 0 {
			latest = p.Versions[len(p.Versions)-1].Version
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Category, truncate(p.Description, 50), latest)
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
	fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION")
	fmt.Fprintln(w, "----\t--------\t-----------")

	found := 0
	for _, p := range reg.Plugins {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Description), q) ||
			strings.Contains(strings.ToLower(p.Category), q) ||
			containsTag(p.Tags, q) {
			fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Category, truncate(p.Description, 60))
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

	fmt.Fprintf(os.Stdout, "Installing %s@%s ...\n", name, versionEntry.Version)

	binaryPath, err := loader.ResolvePluginBinary(ctx, name, versionEntry.Version)
	if err != nil {
		return fmt.Errorf("installing plugin: %w", err)
	}

	installDir := overrideDir
	if installDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		installDir = filepath.Join(home, ".semrel", "plugins")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}

	binaryName := pluginBinaryName(name)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	dest := filepath.Join(installDir, binaryName)

	if err := copyFile(binaryPath, dest, 0o755); err != nil {
		return fmt.Errorf("installing plugin binary: %w", err)
	}

	fmt.Fprintf(os.Stdout, "✓ Installed %s to %s\n", name, dest)
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
