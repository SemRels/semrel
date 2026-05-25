// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Slack notification plugin for semrel.
//
// SlackNotifier sends release announcements to a Slack channel via an
// Incoming Webhook URL. The payload uses Slack's Block Kit for rich
// formatting with context blocks, release notes, and a link to the release.
//
// Configuration example in .semrel.yaml:
//
//	notifications:
//	  slack:
//	    webhook_url: https://hooks.slack.com/services/T.../B.../...
//	    channel: "#releases"
//	    username: semrel-bot
//	    icon_emoji: ":rocket:"
//
// See: https://github.com/SemRels/semrel/issues/19
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SlackConfig holds the Slack notifier configuration.
type SlackConfig struct {
	// WebhookURL is the Slack Incoming Webhook URL (required).
	WebhookURL string
	// Channel overrides the webhook's default channel (e.g. "#releases").
	Channel string
	// Username is the bot display name (default: "semrel").
	Username string
	// IconEmoji is the bot icon emoji (e.g. ":rocket:").
	IconEmoji string
}

// SlackNotifier sends release announcements to a Slack channel.
type SlackNotifier struct {
	cfg    SlackConfig
	client *http.Client
}

// NewSlackNotifier creates a notifier from the given configuration.
func NewSlackNotifier(cfg SlackConfig) *SlackNotifier {
	if cfg.Username == "" {
		cfg.Username = "semrel"
	}
	if cfg.IconEmoji == "" {
		cfg.IconEmoji = ":rocket:"
	}
	return &SlackNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: DefaultTimeout},
	}
}

// slackPayload is the Slack Incoming Webhook JSON body.
type slackPayload struct {
	Channel   string       `json:"channel,omitempty"`
	Username  string       `json:"username,omitempty"`
	IconEmoji string       `json:"icon_emoji,omitempty"`
	Blocks    []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type string      `json:"type"`
	Text *slackText  `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Notify sends a release notification to Slack.
// The message includes the project name, version, release URL, and a
// truncated excerpt of the release notes.
func (n *SlackNotifier) Notify(ctx context.Context, version, changelog, repository string) error {
	blocks := n.buildBlocks(version, changelog, repository)
	payload := slackPayload{
		Channel:   n.cfg.Channel,
		Username:  n.cfg.Username,
		IconEmoji: n.cfg.IconEmoji,
		Blocks:    blocks,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.WebhookURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("slack: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (n *SlackNotifier) buildBlocks(version, changelog, repository string) []slackBlock {
	header := fmt.Sprintf(":rocket: *%s released*", version)
	if repository != "" {
		header = fmt.Sprintf(":rocket: *%s %s released*", repository, version)
	}

	blocks := []slackBlock{
		{Type: "section", Text: &slackText{Type: "mrkdwn", Text: header}},
	}

	if changelog != "" {
		notes := strings.TrimSpace(changelog)
		// Truncate long release notes (Slack text block limit is ~3000 chars)
		if len(notes) > 900 {
			notes = notes[:900] + "…"
		}
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: notes},
		})
	}

	blocks = append(blocks, slackBlock{Type: "divider"})
	return blocks
}
