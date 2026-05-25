// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package jira provides a Jira release tracker that parses Jira issue keys
// from commit messages, creates a Jira release version, and transitions
// matched issues to a "released" status via the Jira REST API v3.
//
// Authentication uses HTTP Basic Auth with a Jira API token:
// https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// issueKeyPattern matches Jira issue keys like ABC-123 or PROJ-4567.
var issueKeyPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9]+-[0-9]+)\b`)

// Client is a Jira REST API v3 client for release tracking operations.
type Client struct {
	baseURL  string
	email    string
	apiToken string
	http     *http.Client
}

// Config configures the Jira client.
type Config struct {
	// BaseURL is the Jira instance URL (e.g. "https://myorg.atlassian.net").
	BaseURL string
	// Email is the Atlassian account email for Basic Auth.
	Email string
	// APIToken is the Jira API token for Basic Auth.
	APIToken string
	// Timeout for HTTP requests (defaults to 30s).
	Timeout time.Duration
}

// NewClient creates a new Jira API client.
func NewClient(cfg Config) *Client {
	t := cfg.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		email:    cfg.Email,
		apiToken: cfg.APIToken,
		http:     &http.Client{Timeout: t},
	}
}

// Version represents a Jira project version (release).
type Version struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	ProjectID   int    `json:"projectId,omitempty"`
	Description string `json:"description,omitempty"`
	Released    bool   `json:"released"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}

// CreateVersion creates a new version in the specified Jira project.
func (c *Client) CreateVersion(ctx context.Context, projectKey, name, description string) (*Version, error) {
	// First resolve the project ID
	projectID, err := c.getProjectID(ctx, projectKey)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"name":        name,
		"projectId":   projectID,
		"description": description,
		"released":    false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("jira: marshal version: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/rest/api/3/version", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jira: create version request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: create version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("create version", resp)
	}

	var ver Version
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return nil, fmt.Errorf("jira: decode version: %w", err)
	}
	return &ver, nil
}

// ReleaseVersion marks a Jira project version as released.
func (c *Client) ReleaseVersion(ctx context.Context, versionID string) error {
	today := time.Now().UTC().Format("2006-01-02")
	payload := map[string]interface{}{
		"released":    true,
		"releaseDate": today,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jira: marshal release version: %w", err)
	}

	url := fmt.Sprintf("%s/rest/api/3/version/%s", c.baseURL, versionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: release version request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira: release version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return c.apiError("release version", resp)
	}
	return nil
}

// TransitionIssue moves a Jira issue to the given transition (by name, e.g. "Done").
func (c *Client) TransitionIssue(ctx context.Context, issueKey, transitionName string) error {
	transitionID, err := c.getTransitionID(ctx, issueKey, transitionName)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jira: marshal transition: %w", err)
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("jira: transition issue request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira: transition issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return c.apiError("transition issue", resp)
	}
	return nil
}

// ExtractIssueKeys parses Jira issue keys (e.g. "PROJ-123") from a list of commit messages.
func ExtractIssueKeys(commits []string) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, msg := range commits {
		for _, m := range issueKeyPattern.FindAllString(msg, -1) {
			upper := strings.ToUpper(m)
			if _, ok := seen[upper]; !ok {
				seen[upper] = struct{}{}
				keys = append(keys, upper)
			}
		}
	}
	return keys
}

// getProjectID resolves a project key (e.g. "PROJ") to its numeric ID.
func (c *Client) getProjectID(ctx context.Context, projectKey string) (int, error) {
	url := fmt.Sprintf("%s/rest/api/3/project/%s", c.baseURL, projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("jira: get project request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("jira: get project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return 0, c.apiError("get project", resp)
	}

	var project struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return 0, fmt.Errorf("jira: decode project: %w", err)
	}

	var id int
	fmt.Sscanf(project.ID, "%d", &id)
	return id, nil
}

// getTransitionID finds the transition ID for the given transition name.
func (c *Client) getTransitionID(ctx context.Context, issueKey, transitionName string) (string, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("jira: get transitions request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("jira: get transitions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", c.apiError("get transitions", resp)
	}

	var result struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("jira: decode transitions: %w", err)
	}

	for _, t := range result.Transitions {
		if strings.EqualFold(t.Name, transitionName) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("jira: transition %q not found for issue %s", transitionName, issueKey)
}

func (c *Client) setAuth(req *http.Request) {
	if c.email != "" {
		req.SetBasicAuth(c.email, c.apiToken)
	}
}

func (c *Client) apiError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("jira: %s: status %d: %s", op, resp.StatusCode, strings.TrimSpace(string(body)))
}
