// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The go-semrel Authors

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set via ldflags at build time.
var version = "dev"

// NewRootCommand returns the root cobra command for go-semrel.
func NewRootCommand() *cobra.Command {
	var dryRun bool
	var configFile string

	root := &cobra.Command{
		Use:   "go-semrel",
		Short: "A Go-based semantic release system with plugin architecture",
		Long: `go-semrel automates software releases by analysing Conventional Commits,
determining the next SemVer version, generating changelogs and invoking
configurable release plugins (git tags, GitHub/GitLab Releases, npm, Docker, Helm, ...).`,
		Version:      version,
		SilenceUsage: true,
	}

	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate the release without making any changes")
	root.PersistentFlags().StringVar(&configFile, "config", ".semrel.yaml", "Path to configuration file")

	root.AddCommand(newReleaseCommand(&dryRun, &configFile))
	root.AddCommand(newLintCommand(&configFile))

	return root
}

func newReleaseCommand(dryRun *bool, configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "release",
		Short: "Run the full release pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("go-semrel %s – release pipeline (dry-run=%v, config=%s)\n", version, *dryRun, *configFile)
			fmt.Println("🚧 Not yet implemented – see https://github.com/GoSemantics/go-semrel/issues/2")
			return nil
		},
	}
}

func newLintCommand(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate commit messages since the last release",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("go-semrel lint (config=%s)\n", *configFile)
			fmt.Println("🚧 Not yet implemented – see https://github.com/GoSemantics/go-semrel/issues/47")
			return nil
		},
	}
}
