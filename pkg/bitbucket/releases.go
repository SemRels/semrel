// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package bitbucket provides a Bitbucket Cloud releases provider that creates
// download entries and annotated tags via the Bitbucket REST API v2.0.
//
// Authentication uses HTTP Basic Auth with an app password:
// https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.bitbucket.org/2.0"
	defaultTimeout = 30 * time.Second
)

// Client is a Bitbucket Cloud API client for release operations.
type Client struct {
	baseURL     string
	workspace   string
	repoSlug    string
	username    string
	appPassword string
	http        *http.Client
}

// Config configures the Bitbucket client.
type Config struct {
	// BaseURL overrides the Bitbucket API base URL (for testing).
	BaseURL string
	// Workspace is the Bitbucket workspace (organisation or user slug).
	Workspace string
	// RepoSlug is the repository slug.
	RepoSlug string
	// Username is the Bitbucket username for Basic Auth.
	Username string
	// AppPassword is the Bitbucket app password for Basic Auth.
	AppPassword string
	// Timeout for HTTP requests (defaults to 30s).
	Timeout time.Duration
}

// NewClient creates a new Bitbucket API client.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	t := cfg.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Client{
		baseURL:     strings.TrimRight(base, "/"),
		workspace:   cfg.Workspace,
		repoSlug:    cfg.RepoSlug,
		username:    cfg.Username,
		appPassword: cfg.AppPassword,
		http:        &http.Client{Timeout: t},
	}
}

// Tag represents a Bitbucket repository tag.
type Tag struct {
	Name   string `json:"name"`
	Target struct {
		Hash string `json:"hash"`
	} `json:"target"`
	Message string `json:"message,omitempty"`
}

// CreateTag creates an annotated tag in the Bitbucket repository.
func (c *Client) CreateTag(ctx context.Context, name, commitHash, message string) (*Tag, error) {
	payload := map[string]interface{}{
		"name":    name,
		"target":  map[string]string{"hash": commitHash},
		"message": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: marshal tag payload: %w", err)
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/refs/tags", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bitbucket: create tag request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: create tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("create tag", resp)
	}

	var tag Tag
	if err := json.NewDecoder(resp.Body).Decode(&tag); err != nil {
		return nil, fmt.Errorf("bitbucket: decode tag response: %w", err)
	}
	return &tag, nil
}

// Download represents a Bitbucket repository download entry.
type Download struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// Links contains download links.
	Links map[string]map[string]string `json:"links,omitempty"`
}

// UploadDownload uploads a file to the Bitbucket repository downloads.
func (c *Client) UploadDownload(ctx context.Context, filePath string) (*Download, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: open file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("files", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("bitbucket: create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, fmt.Errorf("bitbucket: copy file: %w", err)
	}
	mw.Close()

	url := fmt.Sprintf("%s/repositories/%s/%s/downloads", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: upload download request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: upload download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("upload download", resp)
	}

	// 200 = replaced, 201 = created; Bitbucket returns empty body on success
	return &Download{Name: filepath.Base(filePath)}, nil
}

// ListDownloads lists downloads for the repository.
func (c *Client) ListDownloads(ctx context.Context) ([]Download, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/downloads", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: list downloads request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: list downloads: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, c.apiError("list downloads", resp)
	}

	var result struct {
		Values []Download `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("bitbucket: decode downloads: %w", err)
	}
	return result.Values, nil
}

// PipelineVariable represents a Bitbucket repository variable.
type PipelineVariable struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Secured bool   `json:"secured"`
}

// SetPipelineVariable creates or updates a pipeline variable in the repository.
func (c *Client) SetPipelineVariable(ctx context.Context, key, value string, secured bool) error {
	payload := PipelineVariable{Key: key, Value: value, Secured: secured}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bitbucket: marshal variable: %w", err)
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/pipelines_config/variables/", c.baseURL, c.workspace, c.repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bitbucket: set variable request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: set variable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return c.apiError("set pipeline variable", resp)
	}
	return nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.username != "" {
		req.SetBasicAuth(c.username, c.appPassword)
	}
}

func (c *Client) apiError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("bitbucket: %s: status %d: %s", op, resp.StatusCode, strings.TrimSpace(string(body)))
}
