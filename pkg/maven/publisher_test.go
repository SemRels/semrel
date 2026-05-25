// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package maven_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/maven"
)

const samplePOM = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>myapp</artifactId>
  <version>0.1.0</version>
  <packaging>jar</packaging>
</project>
`

func writePOM(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "pom.xml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdatePOMVersion(t *testing.T) {
	dir := t.TempDir()
	path := writePOM(t, dir, samplePOM)

	pom, err := maven.UpdatePOMVersion(path, "2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pom.Version != "2.3.4" {
		t.Errorf("expected version 2.3.4, got %q", pom.Version)
	}
	if pom.ArtifactID != "myapp" {
		t.Errorf("expected artifactId myapp, got %q", pom.ArtifactID)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "<version>2.3.4</version>") {
		t.Error("pom.xml should contain updated version")
	}
}

func TestUpdatePOMVersion_PreservesXML(t *testing.T) {
	dir := t.TempDir()
	path := writePOM(t, dir, samplePOM)

	_, err := maven.UpdatePOMVersion(path, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "<artifactId>myapp</artifactId>") {
		t.Error("pom.xml should preserve artifactId")
	}
	if !strings.Contains(string(data), "<groupId>com.example</groupId>") {
		t.Error("pom.xml should preserve groupId")
	}
}

func TestReadPOM(t *testing.T) {
	dir := t.TempDir()
	path := writePOM(t, dir, samplePOM)

	pom, err := maven.ReadPOM(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pom.GroupID != "com.example" {
		t.Errorf("expected groupId com.example, got %q", pom.GroupID)
	}
	if pom.ArtifactID != "myapp" {
		t.Errorf("expected artifactId myapp, got %q", pom.ArtifactID)
	}
	if pom.Version != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %q", pom.Version)
	}
}

func TestNewPublisher_Defaults(t *testing.T) {
	p := maven.NewPublisher(maven.Config{})
	_ = p
}

func TestIsMavenAvailable(t *testing.T) {
	_ = maven.IsMavenAvailable()
}
