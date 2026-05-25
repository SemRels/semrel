// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package bitbucket_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoSemantics/semrel/pkg/bitbucket"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *bitbucket.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return bitbucket.NewClient(bitbucket.Config{
		BaseURL:     srv.URL,
		Workspace:   "myorg",
		RepoSlug:    "myrepo",
		Username:    "user",
		AppPassword: "pass",
	})
}

func TestNewClient_Defaults(t *testing.T) {
	c := bitbucket.NewClient(bitbucket.Config{
		Workspace: "org",
		RepoSlug:  "repo",
	})
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
}

func TestCreateTag_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/refs/tags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":    "v1.0.0",
			"message": "Release v1.0.0",
		})
	})
	c := newTestClient(t, mux)

	tag, err := c.CreateTag(context.Background(), "v1.0.0", "abc123", "Release v1.0.0")
	if err != nil {
		t.Fatalf("CreateTag() error: %v", err)
	}
	if tag.Name != "v1.0.0" {
		t.Errorf("tag.Name = %q, want %q", tag.Name, "v1.0.0")
	}
}

func TestCreateTag_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/refs/tags", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"tag already exists"}}`, http.StatusConflict)
	})
	c := newTestClient(t, mux)

	_, err := c.CreateTag(context.Background(), "v1.0.0", "abc123", "")
	if err == nil {
		t.Error("CreateTag() should return error on 409")
	}
}

func TestUploadDownload_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})
	c := newTestClient(t, mux)

	// Create a temporary file to upload
	f, err := os.CreateTemp("", "release-*.tar.gz")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.WriteString("fake archive content")
	f.Close()
	defer os.Remove(f.Name())

	dl, err := c.UploadDownload(context.Background(), f.Name())
	if err != nil {
		t.Fatalf("UploadDownload() error: %v", err)
	}
	if dl.Name != filepath.Base(f.Name()) {
		t.Errorf("Download.Name = %q, want %q", dl.Name, filepath.Base(f.Name()))
	}
}

func TestListDownloads_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/downloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"values": []map[string]interface{}{
				{"name": "myrepo-1.0.0.tar.gz", "size": 1024},
				{"name": "myrepo-1.0.0.zip", "size": 2048},
			},
		})
	})
	c := newTestClient(t, mux)

	downloads, err := c.ListDownloads(context.Background())
	if err != nil {
		t.Fatalf("ListDownloads() error: %v", err)
	}
	if len(downloads) != 2 {
		t.Errorf("len(downloads) = %d, want 2", len(downloads))
	}
}

func TestSetPipelineVariable_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/myorg/myrepo/pipelines_config/variables/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})
	c := newTestClient(t, mux)

	err := c.SetPipelineVariable(context.Background(), "VERSION", "1.0.0", false)
	if err != nil {
		t.Fatalf("SetPipelineVariable() error: %v", err)
	}
}
