// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package notify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/notify"
)

func TestMatrixNotifier_Notify_Success(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("expected Authorization Bearer header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"event_id":"$eventid"}`))
	}))
	defer srv.Close()

	n := notify.NewMatrixNotifier(notify.MatrixConfig{
		HomeserverURL: srv.URL,
		RoomID:        "!testroom:matrix.org",
		AccessToken:   "test-token",
	})

	if err := n.Notify(context.Background(), "v1.2.3", "## Changes\n- new feature", "myapp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
	if !strings.Contains(receivedPath, "m.room.message") {
		t.Errorf("expected m.room.message in path, got %s", receivedPath)
	}
}

func TestMatrixNotifier_Notify_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errcode":"M_FORBIDDEN"}`))
	}))
	defer srv.Close()

	n := notify.NewMatrixNotifier(notify.MatrixConfig{
		HomeserverURL: srv.URL,
		RoomID:        "!room:matrix.org",
		AccessToken:   "bad-token",
	})

	if err := n.Notify(context.Background(), "v1.0.0", "", "repo"); err == nil {
		t.Error("expected error for 403 response")
	}
}

func TestMatrixNotifier_Notify_TruncatesLongNotes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"event_id":"$x"}`))
	}))
	defer srv.Close()

	n := notify.NewMatrixNotifier(notify.MatrixConfig{
		HomeserverURL: srv.URL,
		RoomID:        "!room:matrix.org",
		AccessToken:   "tok",
	})

	longNotes := strings.Repeat("y", 3000)
	if err := n.Notify(context.Background(), "v1.0.0", longNotes, "repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatrixNotifier_Notify_NoChangelog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"event_id":"$y"}`))
	}))
	defer srv.Close()

	n := notify.NewMatrixNotifier(notify.MatrixConfig{
		HomeserverURL: srv.URL,
		RoomID:        "!room:matrix.org",
		AccessToken:   "tok",
	})
	// Should succeed with empty changelog
	if err := n.Notify(context.Background(), "v2.0.0", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
