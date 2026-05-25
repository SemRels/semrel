// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Discord and Microsoft Teams notification plugins for semrel.
//
// DiscordNotifier sends release announcements to a Discord channel via
// a webhook URL. The payload uses Discord's Embed API for rich formatting.
//
// TeamsNotifier sends release announcements to a Microsoft Teams channel via
// an Incoming Webhook. The payload uses the Teams Adaptive Card (MessageCard)
// format.
//
// Configuration examples in .semrel.yaml:
//
//	notifications:
//	  discord:
//	    webhook_url: https://discord.com/api/webhooks/...
//	    username: semrel-bot
//	    avatar_url: https://example.com/semrel.png
//	  teams:
//	    webhook_url: https://outlook.office.com/webhook/...
//
// See:
//   - https://github.com/SemRels/semrel/issues/37 (Discord)
//   - https://github.com/SemRels/semrel/issues/38 (Teams)
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
// Discord
// ---------------------------------------------------------------------------

// DiscordConfig holds the configuration for Discord webhook notifications.
type DiscordConfig struct {
	// WebhookURL is the Discord incoming webhook URL (required).
	WebhookURL string
	// Username overrides the webhook's default display name.
	Username string
	// AvatarURL overrides the webhook's default avatar.
	AvatarURL string
	// Timeout is the HTTP client timeout (defaults to DefaultTimeout).
	Timeout time.Duration
}

type discordEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Color       int    `json:"color,omitempty"` // decimal RGB
}

type discordPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []discordEmbed `json:"embeds,omitempty"`
}

// DiscordNotifier sends release notifications to Discord via a webhook.
type DiscordNotifier struct {
	cfg    DiscordConfig
	client *http.Client
}

// NewDiscordNotifier creates a notifier with the provided configuration.
func NewDiscordNotifier(cfg DiscordConfig) *DiscordNotifier {
	return &DiscordNotifier{cfg: cfg, client: &http.Client{}}
}

// Notify sends a release notification to Discord.
func (d *DiscordNotifier) Notify(ctx context.Context, version, changelog, repository string) error {
	// Truncate changelog for the embed (Discord 4096-char limit)
	desc := changelog
	if len(desc) > 3900 {
		desc = desc[:3900] + "\n\n*(truncated)*"
	}

	payload := discordPayload{
		Username:  d.cfg.Username,
		AvatarURL: d.cfg.AvatarURL,
		Embeds: []discordEmbed{
			{
				Title:       fmt.Sprintf("🚀 Released %s", version),
				Description: desc,
				Color:       0x5865F2, // Discord blurple
			},
		},
	}
	if repository != "" {
		payload.Content = fmt.Sprintf("New release of **%s**: `%s`", repository, version)
	}

	return d.post(ctx, payload)
}

func (d *DiscordNotifier) post(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling Discord payload: %w", err)
	}

	timeout := d.cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, d.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building Discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "semrel-discord/1")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("Discord webhook POST: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Microsoft Teams
// ---------------------------------------------------------------------------

// TeamsConfig holds the configuration for Microsoft Teams webhook notifications.
type TeamsConfig struct {
	// WebhookURL is the Teams Incoming Webhook URL (required).
	WebhookURL string
	// Timeout is the HTTP client timeout (defaults to DefaultTimeout).
	Timeout time.Duration
}

// teamsMessageCard is the "legacy" Teams card format (supported by all connectors).
type teamsMessageCard struct {
	Type       string              `json:"@type"`
	Context    string              `json:"@context"`
	ThemeColor string              `json:"themeColor"`
	Summary    string              `json:"summary"`
	Sections   []teamsCardSection  `json:"sections,omitempty"`
}

type teamsCardSection struct {
	ActivityTitle    string         `json:"activityTitle,omitempty"`
	ActivitySubtitle string         `json:"activitySubtitle,omitempty"`
	Facts            []teamsCardFact `json:"facts,omitempty"`
	Text             string          `json:"text,omitempty"`
	Markdown         bool            `json:"markdown"`
}

type teamsCardFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TeamsNotifier sends release notifications to Microsoft Teams via a webhook.
type TeamsNotifier struct {
	cfg    TeamsConfig
	client *http.Client
}

// NewTeamsNotifier creates a notifier with the provided configuration.
func NewTeamsNotifier(cfg TeamsConfig) *TeamsNotifier {
	return &TeamsNotifier{cfg: cfg, client: &http.Client{}}
}

// Notify sends a release notification to Microsoft Teams.
func (t *TeamsNotifier) Notify(ctx context.Context, version, changelog, repository string) error {
	// Truncate changelog for the card text (Teams has limits)
	text := changelog
	if len(text) > 2000 {
		text = text[:2000] + "\n\n*(truncated)*"
	}

	facts := []teamsCardFact{{Name: "Version", Value: version}}
	if repository != "" {
		facts = append(facts, teamsCardFact{Name: "Repository", Value: repository})
	}

	// Strip Markdown headings for cleaner Teams display
	summary := fmt.Sprintf("Released %s", version)
	if repository != "" {
		summary = fmt.Sprintf("%s released %s", repository, version)
	}

	card := teamsMessageCard{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: "0076D7",
		Summary:    summary,
		Sections: []teamsCardSection{
			{
				ActivityTitle:    fmt.Sprintf("🚀 %s", summary),
				ActivitySubtitle: "A new version has been released by semrel",
				Facts:            facts,
				Markdown:         true,
			},
			{
				Text:     strings.ReplaceAll(text, "\n", "\n\n"),
				Markdown: true,
			},
		},
	}

	return t.post(ctx, card)
}

func (t *TeamsNotifier) post(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling Teams payload: %w", err)
	}

	timeout := t.cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, t.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building Teams request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "semrel-teams/1")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("Teams webhook POST: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Teams webhook returned status %d", resp.StatusCode)
	}
	return nil
}
