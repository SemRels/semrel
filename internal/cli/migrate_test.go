// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCommand_RegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Use == "migrate" {
			return
		}
	}
	t.Error("expected 'migrate' command registered in root")
}

func TestRunMigrate_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	err := runMigrate(filepath.Join(dir, "nonexistent.yaml"), false, false)
	if err == nil {
		t.Error("expected error when config file does not exist")
	}
}

func TestRunMigrate_AlreadyCurrent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
schemaVersion: 1
branches:
  - name: main
`)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMigrate(cfgPath, false, false)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf strings.Builder
	buf.Grow(256)
	tmp := make([]byte, 256)
	for {
		n, e := r.Read(tmp)
		buf.Write(tmp[:n])
		if e != nil {
			break
		}
	}
	if !strings.Contains(buf.String(), "already at schema version") {
		t.Errorf("expected up-to-date message, got: %s", buf.String())
	}
}

func TestRunMigrate_Unversioned(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	if err := runMigrate(cfgPath, false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-read file and verify schemaVersion is written.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "schemaVersion") {
		t.Errorf("expected schemaVersion in migrated config, got:\n%s", string(data))
	}
}

func TestRunMigrate_DryRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	original, _ := os.ReadFile(cfgPath)

	if err := runMigrate(cfgPath, true, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, _ := os.ReadFile(cfgPath)
	if string(original) != string(after) {
		t.Error("dry-run should not modify the file")
	}
}

func TestRunMigrate_BackupCreated(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	if err := runMigrate(cfgPath, false, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for backup file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	hasBackup := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".semrel.yaml.bak.") {
			hasBackup = true
		}
	}
	if !hasBackup {
		t.Error("expected a backup file to be created")
	}
}
