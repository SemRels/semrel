// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package githubrelease provides a GitHub Releases publisher plugin.
// It creates GitHub releases, uploads release assets, and manages release metadata
// using the GitHub REST API.
package githubrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second
const defaultBaseURL = "https://api.github.com"

// Client interacts with the GitHub Releases API.
type Client struct {
	baseURL    string
	token      string
	owner      string
	repo       string
	httpClient *http.Client
}

// Config holds the configuration for the GitHub Releases client.
type Config struct {
	// BaseURL is the GitHub API URL (defaults to https://api.github.com).
	// Override for GitHub Enterprise Server.
	BaseURL string
	// Token is a GitHub personal access token with 'contents' write scope.
	Token string
	// Owner is the repository owner (user or organization).
	Owner string
	// Repo is the repository name.
	Repo string
	// Timeout is the HTTP client timeout (defaults to 30s).
	Timeout time.Duration
}

// NewClient creates a Client with the provided configuration.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	t := cfg.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		token:      cfg.Token,
		owner:      cfg.Owner,
		repo:       cfg.Repo,
		httpClient: &http.Client{Timeout: t},
	}
}

// Release represents a GitHub release.
type Release struct {
	ID         int    `json:"id"`
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	HTMLURL    string `json:"html_url"`
	UploadURL  string `json:"upload_url"`
}

// CreateReleaseRequest is the payload for creating a release.
type CreateReleaseRequest struct {
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish,omitempty"`
	Name            string `json:"name,omitempty"`
	Body            string `json:"body,omitempty"`
	Draft           bool   `json:"draft"`
	Prerelease      bool   `json:"prerelease"`
	GenerateNotes   bool   `json:"generate_release_notes,omitempty"`
}

// CreateRelease creates a new GitHub release.
func (c *Client) CreateRelease(ctx context.Context, req CreateReleaseRequest) (*Release, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("githubrelease: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases", c.baseURL, c.owner, c.repo)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("githubrelease: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("githubrelease: create release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("githubrelease: create release: status %d: %s", resp.StatusCode, respBody)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("githubrelease: decode release: %w", err)
	}
	return &rel, nil
}

// UploadAsset uploads a file as a release asset.
// uploadURL is the upload_url from the Release returned by CreateRelease.
func (c *Client) UploadAsset(ctx context.Context, uploadURL, filePath, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("githubrelease: open asset: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("githubrelease: stat asset: %w", err)
	}

	// Strip the {?name,label} URI template suffix
	baseURL := strings.SplitN(uploadURL, "{", 2)[0]
	url := baseURL + "?name=" + filepath.Base(filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, f)
	if err != nil {
		return fmt.Errorf("githubrelease: create upload request: %w", err)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.ContentLength = info.Size()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("githubrelease: upload asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("githubrelease: upload asset: status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// GetRelease retrieves a release by tag name.
func (c *Client) GetRelease(ctx context.Context, tagName string) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s",
		c.baseURL, c.owner, c.repo, tagName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("githubrelease: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("githubrelease: get release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("githubrelease: release %q not found", tagName)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("githubrelease: get release: status %d: %s", resp.StatusCode, respBody)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("githubrelease: decode release: %w", err)
	}
	return &rel, nil
}
