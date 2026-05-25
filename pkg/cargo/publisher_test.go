// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cargo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/cargo"
)

func writeCargoTOML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "Cargo.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateVersion_Basic(t *testing.T) {
	dir := t.TempDir()
	path := writeCargoTOML(t, dir, `[package]
name = "mylib"
version = "0.1.0"
edition = "2021"

[dependencies]
`)

	meta, err := cargo.UpdateVersion(path, "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", meta.Version)
	}
	if meta.Name != "mylib" {
		t.Errorf("expected name mylib, got %q", meta.Name)
	}

	// Verify file was updated
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = "1.2.3"`) {
		t.Error("Cargo.toml should contain updated version")
	}
}

func TestUpdateVersion_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := writeCargoTOML(t, dir, `[package]
name = "myapp"
version = "0.0.1"
edition = "2021"
description = "My app"

[dependencies]
serde = "1.0"
`)

	_, err := cargo.UpdateVersion(path, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `edition = "2021"`) {
		t.Error("UpdateVersion should preserve edition field")
	}
	if !strings.Contains(string(data), `description = "My app"`) {
		t.Error("UpdateVersion should preserve description field")
	}
	if !strings.Contains(string(data), `serde = "1.0"`) {
		t.Error("UpdateVersion should preserve dependencies")
	}
}

func TestUpdateVersion_NoVersionField(t *testing.T) {
	dir := t.TempDir()
	path := writeCargoTOML(t, dir, "[package]\nname = \"broken\"\n")

	_, err := cargo.UpdateVersion(path, "1.0.0")
	if err == nil {
		t.Error("expected error when version field is missing")
	}
}

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeCargoTOML(t, dir, "[package]\nname = \"test\"\nversion = \"3.1.4\"\n")

	version, err := cargo.ReadVersion(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "3.1.4" {
		t.Errorf("expected version 3.1.4, got %q", version)
	}
}

func TestReadVersion_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeCargoTOML(t, dir, "[package]\nname = \"test\"\n")

	_, err := cargo.ReadVersion(path)
	if err == nil {
		t.Error("expected error when version not found")
	}
}

func TestIsCargoAvailable(t *testing.T) {
	_ = cargo.IsCargoAvailable()
}

func TestNewPublisher_Defaults(t *testing.T) {
	p := cargo.NewPublisher(cargo.Config{Token: "tok"})
	_ = p
}
