// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package python_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/python"
)

func writePyproject(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdatePyprojectVersion_PEP621(t *testing.T) {
	dir := t.TempDir()
	path := writePyproject(t, dir, `[project]
name = "mypackage"
version = "0.1.0"
description = "A sample package"
`)

	meta, err := python.UpdatePyprojectVersion(path, "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", meta.Version)
	}
	if meta.Name != "mypackage" {
		t.Errorf("expected name mypackage, got %q", meta.Name)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = "1.2.3"`) {
		t.Error("pyproject.toml should contain updated version")
	}
}

func TestUpdatePyprojectVersion_Poetry(t *testing.T) {
	dir := t.TempDir()
	path := writePyproject(t, dir, `[tool.poetry]
name = "poetry-pkg"
version = "0.2.0"

[tool.poetry.dependencies]
python = "^3.9"
`)

	meta, err := python.UpdatePyprojectVersion(path, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %q", meta.Version)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = "2.0.0"`) {
		t.Error("pyproject.toml should contain updated version")
	}
}

func TestUpdatePyprojectVersion_PreservesFields(t *testing.T) {
	dir := t.TempDir()
	path := writePyproject(t, dir, `[project]
name = "mypkg"
version = "1.0.0"
description = "My package"
requires-python = ">=3.8"

[build-system]
requires = ["setuptools"]
`)

	_, err := python.UpdatePyprojectVersion(path, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `requires-python = ">=3.8"`) {
		t.Error("UpdatePyprojectVersion should preserve requires-python field")
	}
	if !strings.Contains(string(data), `requires = ["setuptools"]`) {
		t.Error("UpdatePyprojectVersion should preserve build-system section")
	}
}

func TestUpdatePyprojectVersion_NoVersion(t *testing.T) {
	dir := t.TempDir()
	path := writePyproject(t, dir, "[project]\nname = \"broken\"\n")

	_, err := python.UpdatePyprojectVersion(path, "1.0.0")
	if err == nil {
		t.Error("expected error when version not found")
	}
}

func TestUpdateSetupCfgVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.cfg")
	os.WriteFile(path, []byte("[metadata]\nname = mypkg\nversion = 0.1.0\n"), 0o644)

	if err := python.UpdateSetupCfgVersion(path, "2.3.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "version = 2.3.4") {
		t.Errorf("setup.cfg should contain updated version, got: %s", data)
	}
}

func TestIsTwineAvailable(t *testing.T) {
	_ = python.IsTwineAvailable()
}

func TestIsPythonAvailable(t *testing.T) {
	_ = python.IsPythonAvailable()
}
