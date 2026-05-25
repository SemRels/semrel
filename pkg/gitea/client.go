// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package gitea provides a Gitea/Forgejo releases client for semrel.
// After a release, semrel can create a Gitea or Forgejo Release using the
// Gitea REST API (Forgejo is API-compatible).
//
// Configuration example in .semrel.yaml:
//
//	plugins:
//	  - uses: builtin:gitea
//	    config:
//	      base_url: https://gitea.example.com
//	      token: ${GITEA_TOKEN}
//	      owner: my-org
//	      repo: my-repo
//	      draft: false
//	      prerelease: false
//
// See: https://github.com/SemRels/semrel/issues/36
package gitea

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

const defaultTimeout = 15 * time.Second

// Config holds the configuration for the Gitea/Forgejo release client.
type Config struct {
	// BaseURL is the Gitea/Forgejo instance URL (e.g. https://gitea.example.com).
	BaseURL string
	// Token is the Gitea API token with write:repository scope.
	Token string
	// Owner is the repository owner (user or org name).
	Owner string
	// Repo is the repository name.
	Repo string
	// Draft creates a draft release when true.
	Draft bool
	// Prerelease marks the release as a pre-release when true.
	Prerelease bool
	// Timeout overrides the default 15s HTTP timeout.
	Timeout time.Duration
}

// Release represents a Gitea/Forgejo release.
type Release struct {
	ID         int64  `json:"id"`
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	HTMLURL    string `json:"html_url"`
}

// createReleaseRequest is the JSON body for POST /repos/{owner}/{repo}/releases.
type createReleaseRequest struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// Client is a Gitea/Forgejo API client for creating releases.
type Client struct {
	cfg    Config
	http   *http.Client
}

// NewClient returns a new Gitea/Forgejo release client.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

// CreateRelease creates a new Gitea/Forgejo release for the given tag.
func (c *Client) CreateRelease(ctx context.Context, tagName, changelog string) (*Release, error) {
	body := createReleaseRequest{
		TagName:    tagName,
		Name:       tagName,
		Body:       changelog,
		Draft:      c.cfg.Draft,
		Prerelease: c.cfg.Prerelease,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling release request: %w", err)
	}

	url := c.apiURL(fmt.Sprintf("/repos/%s/%s/releases", c.cfg.Owner, c.cfg.Repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST release: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var release Release
	if err := json.Unmarshal(respBody, &release); err != nil {
		return nil, fmt.Errorf("decoding release response: %w", err)
	}
	return &release, nil
}

// GetRelease retrieves a release by tag name.
func (c *Client) GetRelease(ctx context.Context, tagName string) (*Release, error) {
	url := c.apiURL(fmt.Sprintf("/repos/%s/%s/releases/tags/%s", c.cfg.Owner, c.cfg.Repo, tagName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET release: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %q not found", tagName)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var release Release
	if err := json.Unmarshal(respBody, &release); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &release, nil
}

// apiURL constructs the full API URL for the given path.
func (c *Client) apiURL(path string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	return base + "/api/v1" + path
}
