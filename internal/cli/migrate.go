// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/GoSemantics/semrel/pkg/config"
)

func newMigrateCommand(configFile *string) *cobra.Command {
	var dryRun bool
	var noBackup bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate .semrel.yaml to the current schema version",
		Long: `migrate upgrades .semrel.yaml from an older schema version to the current one.

Each schema change is captured as a named migration.  When the config already
carries the current schema version, migrate exits 0 without modifying anything.

A timestamped backup (.semrel.yaml.bak.<timestamp>) is written automatically
before any changes are made.  Use --no-backup to suppress this.

Examples:
  semrel migrate               # apply all pending migrations
  semrel migrate --dry-run     # show what would change without writing
  semrel migrate --no-backup   # skip backup (e.g. when under version control)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(*configFile, dryRun, noBackup)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be migrated without writing any files")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false,
		"Do not write a backup before migrating")
	return cmd
}

func runMigrate(configFile string, dryRun bool, noBackup bool) error {
	cfgPath, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fromVersion := cfg.SchemaVersion

	if config.IsUpToDate(cfg) {
		fmt.Printf("✔  %s is already at schema version %d — nothing to migrate\n",
			cfgPath, config.CurrentSchemaVersion)
		return nil
	}

	fmt.Printf("Detected %s at schema version %d\n", cfgPath, fromVersion)
	fmt.Printf("Current schema version: %d\n\n", config.CurrentSchemaVersion)

	applied, err := config.MigrateConfig(cfg)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	if len(applied) == 0 {
		fmt.Println("No migrations to apply.")
		return nil
	}

	fmt.Println("Changes:")
	for _, a := range applied {
		fmt.Printf("  [migration] %s\n", a)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("[dry-run] No files written.")
		return nil
	}

	// Write backup before modifying.
	if !noBackup {
		backupPath := cfgPath + ".bak." + time.Now().Format("20060102150405")
		original, readErr := os.ReadFile(cfgPath)
		if readErr != nil {
			return fmt.Errorf("reading config for backup: %w", readErr)
		}
		if writeErr := os.WriteFile(backupPath, original, 0o644); writeErr != nil {
			return fmt.Errorf("writing backup to %s: %w", backupPath, writeErr)
		}
		fmt.Printf("Backup written to %s\n", filepath.Base(backupPath))
	}

	// Marshal and write the migrated config.
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling migrated config: %w", err)
	}
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		return fmt.Errorf("writing migrated config: %w", err)
	}

	fmt.Printf("✔  %s migrated to schema version %d\n", cfgPath, config.CurrentSchemaVersion)
	return nil
}
