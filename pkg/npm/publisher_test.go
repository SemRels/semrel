// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package npm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/npm"
)

func writePackageJSON(t *testing.T, dir string, content map[string]any) string {
	t.Helper()
	data, _ := json.Marshal(content)
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateVersion(t *testing.T) {
	dir := t.TempDir()
	path := writePackageJSON(t, dir, map[string]any{
		"name":    "my-pkg",
		"version": "1.0.0",
		"description": "test",
	})

	pkg, err := npm.UpdateVersion(path, "2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Version != "2.3.4" {
		t.Errorf("expected version 2.3.4, got %s", pkg.Version)
	}

	// Verify file was updated
	data, _ := os.ReadFile(path)
	var raw map[string]string
	json.Unmarshal(data, &raw)
	if raw["version"] != "2.3.4" {
		t.Errorf("file not updated with new version")
	}
}

func TestUpdateVersion_PreservesFields(t *testing.T) {
	dir := t.TempDir()
	path := writePackageJSON(t, dir, map[string]any{
		"name":    "my-pkg",
		"version": "1.0.0",
		"scripts": map[string]string{"test": "jest"},
		"main":    "index.js",
	})

	_, err := npm.UpdateVersion(path, "1.2.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"main"`) {
		t.Error("UpdateVersion should preserve 'main' field")
	}
	if !strings.Contains(string(data), `"scripts"`) {
		t.Error("UpdateVersion should preserve 'scripts' field")
	}
}

func TestPublisher_PublishTarball_Success(t *testing.T) {
	var received []byte
	var receivedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		receivedMethod = r.Method
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// Create a dummy tarball
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "my-pkg-1.0.0.tgz")
	os.WriteFile(tgzPath, []byte("fake tarball content"), 0o644)

	p := npm.NewPublisher(npm.Config{
		Registry: srv.URL,
		Token:    "test-token",
	})
	pkg := &npm.PackageJSON{Name: "my-pkg", Version: "1.0.0"}

	if err := p.PublishTarball(context.Background(), tgzPath, pkg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
	// Payload should contain package name
	if !strings.Contains(string(received), "my-pkg") {
		t.Error("payload should contain package name")
	}
}

func TestPublisher_PublishTarball_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "pkg-1.0.0.tgz")
	os.WriteFile(tgzPath, []byte("tgz"), 0o644)

	p := npm.NewPublisher(npm.Config{Registry: srv.URL})
	pkg := &npm.PackageJSON{Name: "pkg", Version: "1.0.0"}
	err := p.PublishTarball(context.Background(), tgzPath, pkg)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestPublisher_PublishTarball_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "pkg-1.0.0.tgz")
	os.WriteFile(tgzPath, []byte("tgz"), 0o644)

	p := npm.NewPublisher(npm.Config{Registry: srv.URL})
	pkg := &npm.PackageJSON{Name: "pkg", Version: "1.0.0"}
	if err := p.PublishTarball(context.Background(), tgzPath, pkg); err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestIsNPMAvailable(t *testing.T) {
	// This just verifies the function runs without panic
	_ = npm.IsNPMAvailable()
}

func TestNewPublisher_Defaults(t *testing.T) {
	// Just verify no panic on empty config
	p := npm.NewPublisher(npm.Config{})
	_ = p
}
