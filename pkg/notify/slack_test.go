// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package notify_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoSemantics/semrel/pkg/notify"
)

func TestSlackNotifier_Notify_Success(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content type")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	n := notify.NewSlackNotifier(notify.SlackConfig{
		WebhookURL: srv.URL,
		Channel:    "#releases",
		Username:   "bot",
		IconEmoji:  ":tada:",
	})

	ctx := context.Background()
	if err := n.Notify(ctx, "v1.2.3", "### Added\n- new feature", "myapp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify payload structure
	var payload map[string]any
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload["username"] != "bot" {
		t.Errorf("expected username 'bot', got %v", payload["username"])
	}
	if payload["icon_emoji"] != ":tada:" {
		t.Errorf("expected icon_emoji ':tada:', got %v", payload["icon_emoji"])
	}
}

func TestSlackNotifier_Notify_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := notify.NewSlackNotifier(notify.SlackConfig{WebhookURL: srv.URL})
	if err := n.Notify(context.Background(), "v1.0.0", "", "repo"); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestSlackNotifier_Notify_TruncatesLongNotes(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	n := notify.NewSlackNotifier(notify.SlackConfig{WebhookURL: srv.URL})
	longNotes := strings.Repeat("x", 2000)
	if err := n.Notify(context.Background(), "v1.0.0", longNotes, "repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Payload shouldn't be enormous
	if len(received) > 2500 {
		t.Errorf("expected truncated payload, got %d bytes", len(received))
	}
}

func TestSlackNotifier_Defaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["username"] != "semrel" {
			t.Errorf("expected default username 'semrel', got %v", payload["username"])
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	n := notify.NewSlackNotifier(notify.SlackConfig{WebhookURL: srv.URL})
	n.Notify(context.Background(), "v1.0.0", "", "")
}

func TestSlackNotifier_BlocksContainVersion(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	n := notify.NewSlackNotifier(notify.SlackConfig{WebhookURL: srv.URL})
	n.Notify(context.Background(), "v2.3.4", "release notes here", "myproject")

	if !strings.Contains(string(received), "v2.3.4") {
		t.Error("expected version in Slack payload")
	}
	if !strings.Contains(string(received), "myproject") {
		t.Error("expected repository name in Slack payload")
	}
}
