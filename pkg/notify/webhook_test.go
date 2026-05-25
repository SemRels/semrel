// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// helpers ----------------------------------------------------------------

func makeServer(t *testing.T, statusCode int, handler func(r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler != nil {
			handler(r)
		}
		w.WriteHeader(statusCode)
	}))
}

// tests ------------------------------------------------------------------

func TestWebhookNotifier_Send_Success(t *testing.T) {
	var received WebhookPayload
	srv := makeServer(t, 200, func(r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
	})
	defer srv.Close()

	cfg := WebhookConfig{URL: srv.URL}
	n := NewWebhookNotifier([]WebhookConfig{cfg})
	payload := WebhookPayload{
		Event:      "release.published",
		Version:    "1.2.0",
		Repository: "org/repo",
		Changelog:  "## v1.2.0\n- feat: something",
	}
	if err := n.Notify(context.Background(), payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.Version != "1.2.0" {
		t.Errorf("expected version 1.2.0, got %q", received.Version)
	}
	if received.Repository != "org/repo" {
		t.Errorf("expected org/repo, got %q", received.Repository)
	}
}

func TestWebhookNotifier_ContentType(t *testing.T) {
	var ct string
	srv := makeServer(t, 200, func(r *http.Request) {
		ct = r.Header.Get("Content-Type")
	})
	defer srv.Close()

	n := NewWebhookNotifier([]WebhookConfig{{URL: srv.URL}})
	n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})

	if ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}

func TestWebhookNotifier_HMACSignature(t *testing.T) {
	var sig string
	var body []byte
	srv := makeServer(t, 200, func(r *http.Request) {
		sig = r.Header.Get("X-Hub-Signature-256")
		body, _ = io.ReadAll(r.Body)
	})
	defer srv.Close()

	secret := "mysecret"
	n := NewWebhookNotifier([]WebhookConfig{{URL: srv.URL, Secret: secret}})
	n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})

	if sig == "" {
		t.Fatal("expected X-Hub-Signature-256 header")
	}
	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("signature should start with sha256=, got %q", sig)
	}
	// Verify the signature is correct
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != expected {
		t.Errorf("signature mismatch: got %q, want %q", sig, expected)
	}
}

func TestWebhookNotifier_NoSignatureWhenNoSecret(t *testing.T) {
	var sig string
	srv := makeServer(t, 200, func(r *http.Request) {
		sig = r.Header.Get("X-Hub-Signature-256")
	})
	defer srv.Close()

	n := NewWebhookNotifier([]WebhookConfig{{URL: srv.URL}})
	n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})

	if sig != "" {
		t.Errorf("expected no signature header, got %q", sig)
	}
}

func TestWebhookNotifier_CustomHeaders(t *testing.T) {
	var hval string
	srv := makeServer(t, 200, func(r *http.Request) {
		hval = r.Header.Get("X-My-Header")
	})
	defer srv.Close()

	cfg := WebhookConfig{
		URL:     srv.URL,
		Headers: map[string]string{"X-My-Header": "custom-value"},
	}
	n := NewWebhookNotifier([]WebhookConfig{cfg})
	n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})

	if hval != "custom-value" {
		t.Errorf("expected custom-value, got %q", hval)
	}
}

func TestWebhookNotifier_NonSuccessStatus(t *testing.T) {
	srv := makeServer(t, 500, nil)
	defer srv.Close()

	n := NewWebhookNotifier([]WebhookConfig{{URL: srv.URL}})
	err := n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error message to contain 500, got %q", err.Error())
	}
}

func TestWebhookNotifier_MultipleEndpoints_OneFailure(t *testing.T) {
	srv200 := makeServer(t, 200, nil)
	srv500 := makeServer(t, 500, nil)
	defer srv200.Close()
	defer srv500.Close()

	n := NewWebhookNotifier([]WebhookConfig{
		{URL: srv200.URL},
		{URL: srv500.URL},
	})
	err := n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})
	if err == nil {
		t.Fatal("expected error when one endpoint fails")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got %q", err.Error())
	}
}

func TestWebhookNotifier_MultipleEndpoints_AllSuccess(t *testing.T) {
	count := 0
	srv1 := makeServer(t, 200, func(_ *http.Request) { count++ })
	srv2 := makeServer(t, 201, func(_ *http.Request) { count++ })
	defer srv1.Close()
	defer srv2.Close()

	n := NewWebhookNotifier([]WebhookConfig{
		{URL: srv1.URL},
		{URL: srv2.URL},
	})
	if err := n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 requests, got %d", count)
	}
}

func TestWebhookNotifier_AutoSetsTimestamp(t *testing.T) {
	var received WebhookPayload
	srv := makeServer(t, 200, func(r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
	})
	defer srv.Close()

	n := NewWebhookNotifier([]WebhookConfig{{URL: srv.URL}})
	n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"}) // no Timestamp

	if received.Timestamp == "" {
		t.Error("expected Timestamp to be auto-populated")
	}
	if _, err := time.Parse(time.RFC3339, received.Timestamp); err != nil {
		t.Errorf("Timestamp is not RFC3339: %q", received.Timestamp)
	}
}

func TestWebhookNotifier_Timeout(t *testing.T) {
	// Server that never responds within 1ms
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer slow.Close()

	cfg := WebhookConfig{URL: slow.URL, Timeout: 1 * time.Millisecond}
	n := NewWebhookNotifier([]WebhookConfig{cfg})
	err := n.Notify(context.Background(), WebhookPayload{Version: "1.0.0"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
