// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package gitea

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mockReleaseServer(t *testing.T, statusCode int, response Release) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}))
}

func TestClient_APIURL(t *testing.T) {
	c := NewClient(Config{BaseURL: "https://gitea.example.com/"})
	got := c.apiURL("/repos/org/repo/releases")
	want := "https://gitea.example.com/api/v1/repos/org/repo/releases"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestClient_APIURL_NoTrailingSlash(t *testing.T) {
	c := NewClient(Config{BaseURL: "https://gitea.example.com"})
	got := c.apiURL("/repos/org/repo/releases")
	if !strings.Contains(got, "/api/v1/repos/") {
		t.Errorf("unexpected URL: %q", got)
	}
}

func TestClient_CreateRelease_Success(t *testing.T) {
	srv := mockReleaseServer(t, http.StatusCreated, Release{
		ID:      42,
		TagName: "v1.0.0",
		Name:    "v1.0.0",
		HTMLURL: "https://gitea.example.com/org/repo/releases/tag/v1.0.0",
	})
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Owner: "org", Repo: "repo", Token: "mytoken"})
	rel, err := c.CreateRelease(context.Background(), "v1.0.0", "## v1.0.0\n- feat: add feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.ID != 42 {
		t.Errorf("expected ID=42, got %d", rel.ID)
	}
	if rel.TagName != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %q", rel.TagName)
	}
}

func TestClient_CreateRelease_SendsBody(t *testing.T) {
	var received createReleaseRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Owner: "org", Repo: "repo", Token: "tok", Draft: true})
	c.CreateRelease(context.Background(), "v1.0.0", "my changelog")

	if received.TagName != "v1.0.0" {
		t.Errorf("expected tag_name=v1.0.0, got %q", received.TagName)
	}
	if received.Body != "my changelog" {
		t.Errorf("expected body='my changelog', got %q", received.Body)
	}
	if !received.Draft {
		t.Error("expected draft=true")
	}
}

func TestClient_CreateRelease_SendsAuthHeader(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Token: "my-api-token"})
	c.CreateRelease(context.Background(), "v1.0.0", "")

	if authHeader != "token my-api-token" {
		t.Errorf("expected 'token my-api-token', got %q", authHeader)
	}
}

func TestClient_CreateRelease_NonCreatedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"already exists"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	_, err := c.CreateRelease(context.Background(), "v1.0.0", "")
	if err == nil {
		t.Fatal("expected error for 409 status")
	}
	if !strings.Contains(err.Error(), "409") {
		t.Errorf("expected 409 in error, got %q", err.Error())
	}
}

func TestClient_GetRelease_Success(t *testing.T) {
	srv := mockReleaseServer(t, http.StatusOK, Release{
		ID:      10,
		TagName: "v2.0.0",
	})
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Owner: "org", Repo: "repo"})
	rel, err := c.GetRelease(context.Background(), "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", rel.TagName)
	}
}

func TestClient_GetRelease_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	_, err := c.GetRelease(context.Background(), "v99.0.0")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}
