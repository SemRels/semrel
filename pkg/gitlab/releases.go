// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package gitlab provides a GitLab Releases publisher plugin.
// It creates releases, uploads release assets (generic packages), and attaches
// links to the GitLab Releases API.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second
const defaultBaseURL = "https://gitlab.com"

// Client interacts with the GitLab Releases API.
type Client struct {
	baseURL    string
	token      string
	projectID  string
	httpClient *http.Client
}

// Config holds the configuration for the GitLab client.
type Config struct {
	// BaseURL is the GitLab instance URL (defaults to https://gitlab.com).
	BaseURL string
	// Token is a GitLab personal access token or project access token.
	Token string
	// ProjectID is the numeric project ID or URL-encoded project path
	// (e.g., "42" or "mygroup%2Fmyproject").
	ProjectID string
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
		projectID:  url.PathEscape(cfg.ProjectID),
		httpClient: &http.Client{Timeout: t},
	}
}

// Release represents a GitLab release.
type Release struct {
	Name        string    `json:"name"`
	TagName     string    `json:"tag_name"`
	Description string    `json:"description"`
	ReleasedAt  time.Time `json:"released_at,omitempty"`
}

// CreateReleaseRequest is the payload for creating a release.
type CreateReleaseRequest struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
}

// ReleaseLink represents a link attachment for a release.
type ReleaseLink struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	LinkType string `json:"link_type,omitempty"` // "runbook", "package", "image", "other"
}

// CreateRelease creates a new GitLab release for the given tag.
func (c *Client) CreateRelease(ctx context.Context, req CreateReleaseRequest) (*Release, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: marshal create release: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/releases", c.baseURL, c.projectID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gitlab: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gitlab: create release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("gitlab: create release: status %d: %s", resp.StatusCode, respBody)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("gitlab: decode release: %w", err)
	}
	return &rel, nil
}

// AddReleaseLink attaches a link to an existing release.
func (c *Client) AddReleaseLink(ctx context.Context, tagName string, link ReleaseLink) error {
	body, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("gitlab: marshal link: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/releases/%s/assets/links",
		c.baseURL, c.projectID, url.PathEscape(tagName))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gitlab: create link request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: add release link: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("gitlab: add link: status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// UploadPackageFile uploads a file to the GitLab Generic Packages registry and
// returns the download URL suitable for use as a release asset link.
func (c *Client) UploadPackageFile(ctx context.Context, packageName, version, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("gitlab: open file: %w", err)
	}
	defer f.Close()

	fileName := filepath.Base(filePath)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("gitlab: create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("gitlab: copy file: %w", err)
	}
	w.Close()

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/packages/generic/%s/%s/%s",
		c.baseURL, c.projectID,
		url.PathEscape(packageName),
		url.PathEscape(version),
		url.PathEscape(fileName))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, &buf)
	if err != nil {
		return "", fmt.Errorf("gitlab: create upload request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab: upload package file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("gitlab: upload: status %d: %s", resp.StatusCode, respBody)
	}
	return apiURL, nil
}
