// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package gradle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/gradle"
)

func writeBuildGradle(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "build.gradle")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateVersion_Groovy_DoubleQuote(t *testing.T) {
	dir := t.TempDir()
	path := writeBuildGradle(t, dir, `plugins {
    id 'java'
}
group = 'com.example'
version = "0.1.0"
description = "My project"
`)

	bf, err := gradle.UpdateVersion(path, "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bf.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", bf.Version)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `version = "1.2.3"`) {
		t.Errorf("build.gradle should contain updated version, got:\n%s", data)
	}
}

func TestUpdateVersion_Groovy_SingleQuote(t *testing.T) {
	dir := t.TempDir()
	path := writeBuildGradle(t, dir, "group = 'com.example'\nversion = '0.2.0'\n")

	_, err := gradle.UpdateVersion(path, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "version = '2.0.0'") {
		t.Errorf("expected single-quote version, got: %s", data)
	}
}

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeBuildGradle(t, dir, `group = 'com.example'
version = "3.1.4"
`)
	version, err := gradle.ReadVersion(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "3.1.4" {
		t.Errorf("expected version 3.1.4, got %q", version)
	}
}

func TestReadVersion_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeBuildGradle(t, dir, "group = 'com.example'\n")
	_, err := gradle.ReadVersion(path)
	if err == nil {
		t.Error("expected error when version not found")
	}
}

func TestUpdateVersionInGradleProperties(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gradle.properties")
	os.WriteFile(path, []byte("group=com.example\nversion=0.1.0\njavaVersion=11\n"), 0o644)

	if err := gradle.UpdateVersionInGradleProperties(path, "1.5.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "version=1.5.0") {
		t.Errorf("gradle.properties should contain updated version, got: %s", data)
	}
	if !strings.Contains(string(data), "javaVersion=11") {
		t.Error("should preserve other properties")
	}
}

func TestIsGradleAvailable(t *testing.T) {
	_ = gradle.IsGradleAvailable()
}
