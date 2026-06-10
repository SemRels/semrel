// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package config

import (
	"testing"
)

func TestMigrateConfig_Unversioned(t *testing.T) {
	cfg := &Config{
		Branches:      []BranchConfig{{Name: "main"}},
		SchemaVersion: 0, // unversioned
	}
	applied, err := MigrateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) == 0 {
		t.Error("expected at least one migration to be applied")
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected SchemaVersion=%d after migration, got %d", CurrentSchemaVersion, cfg.SchemaVersion)
	}
}

func TestMigrateConfig_AlreadyCurrent(t *testing.T) {
	cfg := &Config{
		Branches:      []BranchConfig{{Name: "main"}},
		SchemaVersion: CurrentSchemaVersion,
	}
	applied, err := MigrateConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("expected no migrations applied, got: %v", applied)
	}
}

func TestIsUpToDate_Current(t *testing.T) {
	cfg := &Config{SchemaVersion: CurrentSchemaVersion}
	if !IsUpToDate(cfg) {
		t.Error("expected IsUpToDate=true for current version")
	}
}

func TestIsUpToDate_Outdated(t *testing.T) {
	cfg := &Config{SchemaVersion: 0}
	if IsUpToDate(cfg) {
		t.Error("expected IsUpToDate=false for version 0")
	}
}

func TestCurrentSchemaVersionIsPositive(t *testing.T) {
	if CurrentSchemaVersion <= 0 {
		t.Errorf("CurrentSchemaVersion must be positive, got %d", CurrentSchemaVersion)
	}
}
