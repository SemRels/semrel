// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package helm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/helm"
)

func writeChartYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "Chart.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateChartVersion_Basic(t *testing.T) {
	dir := t.TempDir()
	chartYAML := `apiVersion: v2
name: myapp
version: 0.1.0
appVersion: "1.0"
description: My application
`
	path := writeChartYAML(t, dir, chartYAML)

	meta, err := helm.UpdateChartVersion(path, "1.2.3", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", meta.Version)
	}
	if meta.Name != "myapp" {
		t.Errorf("expected name myapp, got %q", meta.Name)
	}

	// Verify file was updated
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "version: 1.2.3") {
		t.Error("file should contain updated version")
	}
}

func TestUpdateChartVersion_WithAppVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeChartYAML(t, dir, "apiVersion: v2\nname: test\nversion: 0.0.1\nappVersion: \"0.0.1\"\n")

	meta, err := helm.UpdateChartVersion(path, "2.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.AppVersion != "2.0.0" {
		t.Errorf("expected appVersion 2.0.0, got %q", meta.AppVersion)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "appVersion: 2.0.0") {
		t.Error("file should contain updated appVersion")
	}
}

func TestReadChartMeta(t *testing.T) {
	dir := t.TempDir()
	path := writeChartYAML(t, dir, "apiVersion: v2\nname: testchart\nversion: 3.1.4\n")

	meta, err := helm.ReadChartMeta(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Name != "testchart" {
		t.Errorf("expected name testchart, got %q", meta.Name)
	}
	if meta.Version != "3.1.4" {
		t.Errorf("expected version 3.1.4, got %q", meta.Version)
	}
}

func TestChartYAMLPath(t *testing.T) {
	path := helm.ChartYAMLPath("/charts/myapp")
	if path != filepath.Join("/charts/myapp", "Chart.yaml") {
		t.Errorf("unexpected path: %q", path)
	}
}

func TestPublisher_UploadToHTTPRepo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/charts") {
			t.Errorf("expected /api/charts path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Create a fake chart .tgz
	dir := t.TempDir()
	chartTGZ := filepath.Join(dir, "myapp-1.0.0.tgz")
	os.WriteFile(chartTGZ, []byte("fake chart tgz"), 0o644)

	p := helm.NewPublisher(helm.PublisherConfig{RegistryURL: srv.URL})
	if err := p.UploadToHTTPRepo(context.Background(), chartTGZ); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublisher_UploadToHTTPRepo_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	dir := t.TempDir()
	chartTGZ := filepath.Join(dir, "myapp-1.0.0.tgz")
	os.WriteFile(chartTGZ, []byte("fake"), 0o644)

	p := helm.NewPublisher(helm.PublisherConfig{RegistryURL: srv.URL})
	err := p.UploadToHTTPRepo(context.Background(), chartTGZ)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestIsHelmAvailable(t *testing.T) {
	_ = helm.IsHelmAvailable()
}
