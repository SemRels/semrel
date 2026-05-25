// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package githubrelease_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/githubrelease"
)

func newTestClient(t *testing.T, srv *httptest.Server) *githubrelease.Client {
	t.Helper()
	return githubrelease.NewClient(githubrelease.Config{
		BaseURL: srv.URL,
		Token:   "test-token",
		Owner:   "myorg",
		Repo:    "myrepo",
	})
}

func TestCreateRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(githubrelease.Release{
			ID:        1,
			TagName:   "v1.2.3",
			Name:      "Release v1.2.3",
			HTMLURL:   "https://github.com/myorg/myrepo/releases/tag/v1.2.3",
			UploadURL: "https://uploads.github.com/repos/myorg/myrepo/releases/1/assets{?name,label}",
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rel, err := c.CreateRelease(context.Background(), githubrelease.CreateReleaseRequest{
		TagName: "v1.2.3",
		Name:    "Release v1.2.3",
		Body:    "## Changelog\n- feature A",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Errorf("expected tag v1.2.3, got %q", rel.TagName)
	}
}

func TestCreateRelease_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Validation Failed"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateRelease(context.Background(), githubrelease.CreateReleaseRequest{TagName: "v0.0.0"})
	if err == nil {
		t.Error("expected error for 422 response")
	}
}

func TestGetRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "v1.0.0") {
			t.Errorf("expected tag in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(githubrelease.Release{
			ID:      42,
			TagName: "v1.0.0",
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rel, err := c.GetRelease(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.ID != 42 {
		t.Errorf("expected ID 42, got %d", rel.ID)
	}
}

func TestGetRelease_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetRelease(context.Background(), "v9.9.9")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestUploadAsset_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "asset.tar.gz"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	assetPath := filepath.Join(dir, "asset.tar.gz")
	os.WriteFile(assetPath, []byte("fake archive"), 0o644)

	c := newTestClient(t, srv)
	uploadURL := srv.URL + "/repos/myorg/myrepo/releases/1/assets{?name,label}"
	if err := c.UploadAsset(context.Background(), uploadURL, assetPath, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := githubrelease.NewClient(githubrelease.Config{
		Token: "tok",
		Owner: "owner",
		Repo:  "repo",
	})
	_ = c
}
