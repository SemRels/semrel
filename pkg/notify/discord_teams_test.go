// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Discord tests
// ---------------------------------------------------------------------------

func TestDiscordNotifier_Send_Success(t *testing.T) {
	var received discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(204) // Discord returns 204 on success
	}))
	defer srv.Close()

	n := NewDiscordNotifier(DiscordConfig{WebhookURL: srv.URL, Username: "semrel-bot"})
	if err := n.Notify(context.Background(), "v1.2.0", "## v1.2.0\n- feat: added X", "org/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.Username != "semrel-bot" {
		t.Errorf("expected username semrel-bot, got %q", received.Username)
	}
	if len(received.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(received.Embeds))
	}
	if !strings.Contains(received.Embeds[0].Title, "v1.2.0") {
		t.Errorf("expected version in embed title, got %q", received.Embeds[0].Title)
	}
}

func TestDiscordNotifier_NonSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}))
	defer srv.Close()

	n := NewDiscordNotifier(DiscordConfig{WebhookURL: srv.URL})
	err := n.Notify(context.Background(), "v1.0.0", "", "")
	if err == nil {
		t.Fatal("expected error for 400 status")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got %q", err.Error())
	}
}

func TestDiscordNotifier_TruncatesLongChangelog(t *testing.T) {
	var received discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	longChangelog := strings.Repeat("x", 5000)
	n := NewDiscordNotifier(DiscordConfig{WebhookURL: srv.URL})
	n.Notify(context.Background(), "v1.0.0", longChangelog, "")

	if len(received.Embeds) > 0 && len(received.Embeds[0].Description) > 4096 {
		t.Error("changelog should be truncated for Discord embed")
	}
}

func TestDiscordNotifier_ContentWhenRepo(t *testing.T) {
	var received discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	n := NewDiscordNotifier(DiscordConfig{WebhookURL: srv.URL})
	n.Notify(context.Background(), "v1.0.0", "", "myorg/myrepo")
	if !strings.Contains(received.Content, "myorg/myrepo") {
		t.Errorf("expected repository in content, got %q", received.Content)
	}
}

// ---------------------------------------------------------------------------
// Teams tests
// ---------------------------------------------------------------------------

func TestTeamsNotifier_Send_Success(t *testing.T) {
	var received teamsMessageCard
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := NewTeamsNotifier(TeamsConfig{WebhookURL: srv.URL})
	if err := n.Notify(context.Background(), "v2.0.0", "## v2.0.0\n- breaking!", "org/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.Type != "MessageCard" {
		t.Errorf("expected MessageCard type, got %q", received.Type)
	}
	if !strings.Contains(received.Summary, "v2.0.0") {
		t.Errorf("expected version in summary, got %q", received.Summary)
	}
}

func TestTeamsNotifier_IncludesFacts(t *testing.T) {
	var received teamsMessageCard
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := NewTeamsNotifier(TeamsConfig{WebhookURL: srv.URL})
	n.Notify(context.Background(), "v1.0.0", "", "org/repo")

	var hasVersion, hasRepo bool
	for _, s := range received.Sections {
		for _, f := range s.Facts {
			if f.Name == "Version" && f.Value == "v1.0.0" {
				hasVersion = true
			}
			if f.Name == "Repository" && f.Value == "org/repo" {
				hasRepo = true
			}
		}
	}
	if !hasVersion {
		t.Error("expected Version fact")
	}
	if !hasRepo {
		t.Error("expected Repository fact")
	}
}

func TestTeamsNotifier_NonSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	n := NewTeamsNotifier(TeamsConfig{WebhookURL: srv.URL})
	err := n.Notify(context.Background(), "v1.0.0", "", "")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}
