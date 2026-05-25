// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package gitlab_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/gitlab"
)

func newTestClient(t *testing.T, srv *httptest.Server) *gitlab.Client {
	t.Helper()
	return gitlab.NewClient(gitlab.Config{
		BaseURL:   srv.URL,
		Token:     "test-token",
		ProjectID: "42",
	})
}

func TestCreateRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Error("expected PRIVATE-TOKEN header")
		}
		rel := map[string]string{
			"name":     "Release v1.2.3",
			"tag_name": "v1.2.3",
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rel, err := c.CreateRelease(context.Background(), gitlab.CreateReleaseRequest{
		Name:        "Release v1.2.3",
		TagName:     "v1.2.3",
		Description: "## Changelog\n- feature A",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Errorf("expected tag_name v1.2.3, got %q", rel.TagName)
	}
}

func TestCreateRelease_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"tag not found"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateRelease(context.Background(), gitlab.CreateReleaseRequest{
		Name:    "v9.9.9",
		TagName: "v9.9.9",
	})
	if err == nil {
		t.Fatal("expected error for 422 response")
	}
}

func TestAddReleaseLink_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/links") {
			t.Errorf("expected /links path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"name": "myapp"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.AddReleaseLink(context.Background(), "v1.2.3", gitlab.ReleaseLink{
		Name:     "myapp-linux-amd64.tar.gz",
		URL:      "https://example.com/myapp-v1.2.3-linux-amd64.tar.gz",
		LinkType: "package",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddReleaseLink_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.AddReleaseLink(context.Background(), "v9.9.9", gitlab.ReleaseLink{
		Name: "asset",
		URL:  "https://example.com/asset",
	})
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestUploadPackageFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "myapp") {
			t.Errorf("expected package name in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "201 Created"})
	}))
	defer srv.Close()

	// Create a temp file to upload
	dir := t.TempDir()
	filePath := filepath.Join(dir, "myapp-v1.0.0-linux-amd64.tar.gz")
	os.WriteFile(filePath, []byte("fake archive content"), 0o644)

	c := newTestClient(t, srv)
	downloadURL, err := c.UploadPackageFile(context.Background(), "myapp", "v1.0.0", filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := gitlab.NewClient(gitlab.Config{
		Token:     "tok",
		ProjectID: "mygroup/myproject",
	})
	_ = c
}
