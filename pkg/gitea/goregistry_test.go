// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package gitea_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/gitea"
)

func newGoRegistryConfig(baseURL string) gitea.GoRegistryConfig {
	return gitea.GoRegistryConfig{
		BaseURL:    baseURL,
		Token:      "test-token",
		Owner:      "myorg",
		ModulePath: "github.com/myorg/myapp",
		Version:    "1.2.3",
	}
}

func TestPublishModule_Success(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("expected token auth header")
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	p := gitea.NewGoRegistryPublisher(newGoRegistryConfig(srv.URL))
	if err := p.PublishModule([]byte("zip content"), []byte("module github.com/myorg/myapp\n\ngo 1.24\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 upload calls (zip + mod), got %d", calls)
	}
}

func TestPublishModule_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	p := gitea.NewGoRegistryPublisher(newGoRegistryConfig(srv.URL))
	err := p.PublishModule([]byte("zip"), []byte("mod"))
	if err == nil {
		t.Fatal("expected error for 409 conflict")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestPublishModule_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := gitea.NewGoRegistryPublisher(newGoRegistryConfig(srv.URL))
	err := p.PublishModule([]byte("zip"), []byte("mod"))
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestPackageURL(t *testing.T) {
	p := gitea.NewGoRegistryPublisher(gitea.GoRegistryConfig{
		BaseURL:    "https://gitea.example.com",
		Owner:      "myorg",
		ModulePath: "github.com/myorg/myapp",
		Version:    "1.2.3",
	})
	url := p.PackageURL()
	if !strings.Contains(url, "gitea.example.com") {
		t.Error("expected base URL in package URL")
	}
	if !strings.Contains(url, "v1.2.3") {
		t.Error("expected version in package URL")
	}
	if !strings.Contains(url, "myorg") {
		t.Error("expected owner in package URL")
	}
}

func TestNewGoRegistryPublisher_NotNil(t *testing.T) {
	p := gitea.NewGoRegistryPublisher(gitea.GoRegistryConfig{})
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
}
