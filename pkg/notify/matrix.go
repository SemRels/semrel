// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Matrix
// ---------------------------------------------------------------------------

// MatrixConfig holds the configuration for Matrix room notifications.
type MatrixConfig struct {
	// HomeserverURL is the Matrix homeserver URL (e.g., https://matrix.org).
	HomeserverURL string
	// RoomID is the Matrix room ID (e.g., "!roomid:matrix.org").
	RoomID string
	// AccessToken is the Matrix access token for authentication.
	AccessToken string
	// Timeout is the HTTP client timeout (defaults to DefaultTimeout).
	Timeout time.Duration
}

type matrixContent struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	Format        string `json:"format,omitempty"`
	FormattedBody string `json:"formatted_body,omitempty"`
}

// MatrixNotifier sends release notifications to a Matrix room.
type MatrixNotifier struct {
	cfg    MatrixConfig
	client *http.Client
}

// NewMatrixNotifier creates a notifier with the provided configuration.
func NewMatrixNotifier(cfg MatrixConfig) *MatrixNotifier {
	t := cfg.Timeout
	if t == 0 {
		t = DefaultTimeout
	}
	return &MatrixNotifier{cfg: cfg, client: &http.Client{Timeout: t}}
}

// Notify sends a release notification to a Matrix room.
func (n *MatrixNotifier) Notify(ctx context.Context, version, changelog, repository string) error {
	body := buildMatrixBody(version, changelog, repository)
	html := buildMatrixHTML(version, changelog, repository)

	content := matrixContent{
		MsgType:       "m.text",
		Body:          body,
		Format:        "org.matrix.custom.html",
		FormattedBody: html,
	}

	payload, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("matrix: marshal event: %w", err)
	}

	// Use a timestamp as the transaction ID for idempotency
	txnID := fmt.Sprintf("semrel-%d", time.Now().UnixMilli())
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		strings.TrimRight(n.cfg.HomeserverURL, "/"),
		n.cfg.RoomID,
		txnID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("matrix: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.cfg.AccessToken)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("matrix: send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("matrix: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func buildMatrixBody(version, changelog, repository string) string {
	header := fmt.Sprintf("🚀 %s released", version)
	if repository != "" {
		header = fmt.Sprintf("🚀 %s %s released", repository, version)
	}
	if changelog == "" {
		return header
	}
	notes := strings.TrimSpace(changelog)
	if len(notes) > 1000 {
		notes = notes[:1000] + "\n…(truncated)"
	}
	return header + "\n\n" + notes
}

func buildMatrixHTML(version, changelog, repository string) string {
	header := fmt.Sprintf("<strong>🚀 %s released</strong>", version)
	if repository != "" {
		header = fmt.Sprintf("<strong>🚀 %s %s released</strong>", repository, version)
	}
	if changelog == "" {
		return header
	}
	notes := strings.TrimSpace(changelog)
	if len(notes) > 1000 {
		notes = notes[:1000] + "\n…(truncated)"
	}
	return header + "<br/><pre>" + notes + "</pre>"
}
