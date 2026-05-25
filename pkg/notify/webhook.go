// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package notify provides generic HTTP webhook notification support for semrel.
// After a successful release, semrel can POST a JSON payload to any configured
// HTTP endpoint — useful for triggering CI pipelines, chat bots, or custom
// automation that cannot use native GitHub events.
//
// Webhook configuration example in .semrel.yaml:
//
//	notifications:
//	  webhooks:
//	    - url: https://hooks.example.com/release
//	      secret: ${WEBHOOK_SECRET}          # HMAC-SHA256 signing key (optional)
//	      timeout: 10s
//	      headers:
//	        X-Custom-Header: my-value
//
// See: https://github.com/SemRels/semrel/issues/39
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultTimeout is the HTTP client timeout applied when the caller does not
// set one explicitly.
const DefaultTimeout = 10 * time.Second

// WebhookPayload is the JSON body sent to webhook endpoints.
type WebhookPayload struct {
	// Event is always "release.published".
	Event string `json:"event"`
	// Version is the newly released semver string (e.g. "1.2.0").
	Version string `json:"version"`
	// Repository is the full slug "owner/repo" of the source repository.
	Repository string `json:"repository"`
	// Changelog is the rendered release notes (Markdown).
	Changelog string `json:"changelog"`
	// Timestamp is the UTC time of the release in RFC 3339 format.
	Timestamp string `json:"timestamp"`
}

// WebhookConfig holds the configuration for a single webhook endpoint.
type WebhookConfig struct {
	// URL is the endpoint to POST to (required).
	URL string
	// Secret is an optional HMAC-SHA256 signing key.
	// When set the request carries a X-Hub-Signature-256 header.
	Secret string
	// Timeout overrides DefaultTimeout when non-zero.
	Timeout time.Duration
	// Headers is a map of extra request headers.
	Headers map[string]string
}

// WebhookNotifier sends release notifications to one or more HTTP endpoints.
type WebhookNotifier struct {
	client  *http.Client
	configs []WebhookConfig
}

// NewWebhookNotifier creates a notifier that sends to all provided endpoints.
// The HTTP client can be nil; DefaultTimeout will be used in that case.
func NewWebhookNotifier(configs []WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{
		client:  &http.Client{},
		configs: configs,
	}
}

// Notify sends the release payload to every configured webhook endpoint.
// Errors from individual endpoints are collected and returned as a combined
// error. Successful deliveries are not affected by failed ones.
func (w *WebhookNotifier) Notify(ctx context.Context, payload WebhookPayload) error {
	if payload.Timestamp == "" {
		payload.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if payload.Event == "" {
		payload.Event = "release.published"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling webhook payload: %w", err)
	}

	var errs []error
	for _, cfg := range w.configs {
		if err := w.send(ctx, cfg, body); err != nil {
			errs = append(errs, fmt.Errorf("webhook %s: %w", cfg.URL, err))
		}
	}

	if len(errs) > 0 {
		return joinErrors(errs)
	}
	return nil
}

// send delivers the payload body to a single endpoint.
func (w *WebhookNotifier) send(ctx context.Context, cfg WebhookConfig, body []byte) error {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "semrel-webhook/1")

	// HMAC-SHA256 signing (GitHub-style)
	if cfg.Secret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write(body)
		sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", sig)
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST: %w", err)
	}
	defer resp.Body.Close()
	// Drain body to allow connection reuse
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// joinErrors concatenates multiple errors into a single error.
func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	msg := errs[0].Error()
	for _, e := range errs[1:] {
		msg += "; " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
