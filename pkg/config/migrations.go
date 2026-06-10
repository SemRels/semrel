// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package config_migrations provides composable schema migration functions for
// .semrel.yaml.  Each migration upgrades the config from one schema version to
// the next.  semrel migrate chains them automatically.
package config

import "fmt"

// Migration describes a schema upgrade step from one version to the next.
type Migration struct {
	// FromVersion is the schema version this migration upgrades FROM.
	FromVersion int
	// ToVersion is the schema version this migration upgrades TO.
	ToVersion int
	// Description is a short human-readable description of the changes.
	Description string
	// Apply performs the in-place migration on cfg.
	Apply func(cfg *Config) error
}

// Migrations is the ordered registry of all known schema migrations.
// Add new migrations here when the schema changes.
var Migrations = []Migration{
	// v0 → v1: stamp schemaVersion field (no structural changes in v1).
	{
		FromVersion: 0,
		ToVersion:   1,
		Description: "Stamp schemaVersion: 1 (baseline; no structural changes)",
		Apply: func(cfg *Config) error {
			cfg.SchemaVersion = 1
			return nil
		},
	},
}

// MigrateConfig applies all pending migrations to bring cfg from its current
// schema version up to CurrentSchemaVersion.  Returns a list of applied
// migration descriptions (empty if already up-to-date).
func MigrateConfig(cfg *Config) ([]string, error) {
	current := cfg.SchemaVersion
	if current == 0 {
		// Treat missing schemaVersion as version 0 (pre-versioning).
		current = 0
	}

	var applied []string
	for _, m := range Migrations {
		if m.FromVersion == current {
			if err := m.Apply(cfg); err != nil {
				return applied, fmt.Errorf("migration v%d→v%d failed: %w", m.FromVersion, m.ToVersion, err)
			}
			applied = append(applied, fmt.Sprintf("v%d → v%d: %s", m.FromVersion, m.ToVersion, m.Description))
			current = m.ToVersion
		}
	}
	return applied, nil
}

// IsUpToDate returns true when cfg.SchemaVersion equals CurrentSchemaVersion.
func IsUpToDate(cfg *Config) bool {
	v := cfg.SchemaVersion
	if v == 0 {
		v = 0 // treated as unversioned; 0 < CurrentSchemaVersion always triggers migrate
	}
	return v >= CurrentSchemaVersion
}
