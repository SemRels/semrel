// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package npm provides an npm registry publisher plugin. It updates the
// version field in package.json and publishes the package to a registry
// (defaults to https://registry.npmjs.org) using the npm CLI or a configurable
// HTTP transport for environments where npm is not installed.
package npm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const defaultRegistry = "https://registry.npmjs.org"
const defaultTimeout = 30 * time.Second

// Publisher publishes an npm package to a registry.
type Publisher struct {
	cfg    Config
	client *http.Client
}

// Config holds the publishing configuration.
type Config struct {
	// Registry is the npm registry URL (defaults to https://registry.npmjs.org).
	Registry string
	// Token is the npm access/auth token (Bearer authentication).
	Token string
	// Tag is the dist-tag to apply when publishing (defaults to "latest").
	Tag string
	// Access controls package visibility: "public" or "restricted" (defaults to "public").
	Access string
	// Timeout is the HTTP client timeout (defaults to 30s).
	Timeout time.Duration
	// UseNPMCLI forces use of the npm CLI instead of the built-in HTTP publisher.
	UseNPMCLI bool
}

// NewPublisher creates a Publisher with the given configuration.
func NewPublisher(cfg Config) *Publisher {
	if cfg.Registry == "" {
		cfg.Registry = defaultRegistry
	}
	if cfg.Tag == "" {
		cfg.Tag = "latest"
	}
	if cfg.Access == "" {
		cfg.Access = "public"
	}
	t := cfg.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Publisher{cfg: cfg, client: &http.Client{Timeout: t}}
}

// PackageJSON represents the minimal fields of a package.json file.
type PackageJSON struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Main        string            `json:"main,omitempty"`
	Scripts     map[string]string `json:"scripts,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

// UpdateVersion reads a package.json file, updates the version field, and
// writes it back. Returns the updated PackageJSON.
func UpdateVersion(path, version string) (*PackageJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("npm: read package.json: %w", err)
	}

	// Unmarshal into a raw map to preserve unknown fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("npm: parse package.json: %w", err)
	}

	raw["version"] = json.RawMessage(fmt.Sprintf("%q", version))

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("npm: marshal package.json: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return nil, fmt.Errorf("npm: write package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(out, &pkg); err != nil {
		return nil, fmt.Errorf("npm: decode updated package.json: %w", err)
	}
	return &pkg, nil
}

// PublishTarball publishes a pre-built .tgz tarball to the registry via HTTP.
// This method does not require the npm CLI to be installed.
func (p *Publisher) PublishTarball(ctx context.Context, tarballPath string, pkg *PackageJSON) error {
	data, err := os.ReadFile(tarballPath)
	if err != nil {
		return fmt.Errorf("npm: read tarball: %w", err)
	}

	// Minimal CouchDB-style publish document
	doc := map[string]any{
		"_id":  pkg.Name,
		"name": pkg.Name,
		"dist-tags": map[string]string{
			p.cfg.Tag: pkg.Version,
		},
		"versions": map[string]any{
			pkg.Version: map[string]any{
				"name":    pkg.Name,
				"version": pkg.Version,
				"dist": map[string]any{
					"tarball": fmt.Sprintf("%s/%s/-/%s-%s.tgz",
						p.cfg.Registry, pkg.Name, pkg.Name, pkg.Version),
				},
			},
		},
		"_attachments": map[string]any{
			fmt.Sprintf("%s-%s.tgz", pkg.Name, pkg.Version): map[string]any{
				"content_type": "application/octet-stream",
				"data":         data,
				"length":       len(data),
			},
		},
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("npm: marshal publish document: %w", err)
	}

	url := fmt.Sprintf("%s/%s", p.cfg.Registry, pkg.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("npm: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.Token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("npm: publish request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("npm: version %s already exists (409 Conflict)", pkg.Version)
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("npm: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// PublishCLI runs "npm publish" using the npm CLI. This method requires npm
// to be installed and a token configured in .npmrc or the NPM_TOKEN env var.
func (p *Publisher) PublishCLI(ctx context.Context, packageDir string) error {
	args := []string{"publish", "--access", p.cfg.Access, "--tag", p.cfg.Tag}
	if p.cfg.Registry != defaultRegistry {
		args = append(args, "--registry", p.cfg.Registry)
	}

	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = packageDir
	env := os.Environ()
	if p.cfg.Token != "" {
		env = append(env, "NPM_TOKEN="+p.cfg.Token)
	}
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm: cli publish: %w\n%s", err, out)
	}
	return nil
}

// IsNPMAvailable reports whether the npm CLI is installed.
func IsNPMAvailable() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}
