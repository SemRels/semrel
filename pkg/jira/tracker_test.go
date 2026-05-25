// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package jira_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoSemantics/semrel/pkg/jira"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *jira.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return jira.NewClient(jira.Config{
		BaseURL:  srv.URL,
		Email:    "user@example.com",
		APIToken: "test-token",
	})
}

func TestExtractIssueKeys(t *testing.T) {
	commits := []string{
		"fix: resolve ABC-123 crash on startup",
		"feat: implement PROJ-456 dark mode",
		"chore: update deps (no jira issue)",
		"fix: ABC-123 duplicate should be deduped",
		"refactor: PROJ-789 and PROJ-456 together",
	}
	keys := jira.ExtractIssueKeys(commits)

	expected := map[string]bool{"ABC-123": true, "PROJ-456": true, "PROJ-789": true}
	if len(keys) != len(expected) {
		t.Errorf("ExtractIssueKeys() returned %d keys, want %d: %v", len(keys), len(expected), keys)
	}
	for _, k := range keys {
		if !expected[k] {
			t.Errorf("unexpected key %q", k)
		}
	}
}

func TestExtractIssueKeys_Empty(t *testing.T) {
	keys := jira.ExtractIssueKeys([]string{"fix: no ticket here", "chore: boring update"})
	if len(keys) != 0 {
		t.Errorf("ExtractIssueKeys() = %v, want empty", keys)
	}
}

func TestCreateVersion_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/project/PROJ", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "10001"})
	})
	mux.HandleFunc("/rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "20001",
			"name": "v1.0.0",
		})
	})
	c := newTestClient(t, mux)

	ver, err := c.CreateVersion(context.Background(), "PROJ", "v1.0.0", "Release v1.0.0")
	if err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}
	if ver.ID != "20001" || ver.Name != "v1.0.0" {
		t.Errorf("CreateVersion() = %+v, want ID=20001, Name=v1.0.0", ver)
	}
}

func TestReleaseVersion_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/version/20001", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "20001", "released": true})
	})
	c := newTestClient(t, mux)

	if err := c.ReleaseVersion(context.Background(), "20001"); err != nil {
		t.Fatalf("ReleaseVersion() error: %v", err)
	}
}

func TestTransitionIssue_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue/PROJ-123/transitions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []map[string]interface{}{
					{"id": "11", "name": "To Do"},
					{"id": "21", "name": "In Progress"},
					{"id": "31", "name": "Done"},
				},
			})
		case http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})
	c := newTestClient(t, mux)

	if err := c.TransitionIssue(context.Background(), "PROJ-123", "Done"); err != nil {
		t.Fatalf("TransitionIssue() error: %v", err)
	}
}

func TestTransitionIssue_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue/PROJ-123/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []map[string]interface{}{
					{"id": "11", "name": "To Do"},
				},
			})
		}
	})
	c := newTestClient(t, mux)

	if err := c.TransitionIssue(context.Background(), "PROJ-123", "Released"); err == nil {
		t.Error("TransitionIssue() should fail when transition not found")
	}
}
