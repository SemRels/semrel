// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package helm provides Helm chart versioning and publishing.
// It updates Chart.yaml version fields and can package and push charts
// to an OCI registry (Helm 3.8+ oci:// protocol) or a classic HTTP chart repository.
package helm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultTimeout = 30 * time.Second

// ChartMeta represents the minimal fields of a Helm Chart.yaml file.
type ChartMeta struct {
	APIVersion  string `yaml:"apiVersion"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// UpdateChartVersion reads a Chart.yaml file, updates the version (and
// optionally appVersion) fields, and writes the file back. It preserves
// comments and ordering by doing a targeted string replacement.
func UpdateChartVersion(chartYAMLPath, version, appVersion string) (*ChartMeta, error) {
	data, err := os.ReadFile(chartYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("helm: read Chart.yaml: %w", err)
	}

	var meta ChartMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("helm: parse Chart.yaml: %w", err)
	}

	// Replace version field in raw YAML (preserves comments/ordering)
	data = replaceYAMLField(data, "version", version)
	if appVersion != "" {
		data = replaceYAMLField(data, "appVersion", appVersion)
		meta.AppVersion = appVersion
	}
	meta.Version = version

	if err := os.WriteFile(chartYAMLPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("helm: write Chart.yaml: %w", err)
	}
	return &meta, nil
}

// replaceYAMLField replaces a YAML scalar field value in raw bytes.
func replaceYAMLField(data []byte, field, value string) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), field+":") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + field + ": " + value
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// Publisher publishes Helm charts to an OCI registry or HTTP repository.
type Publisher struct {
	cfg    PublisherConfig
	client *http.Client
}

// PublisherConfig configures chart publishing.
type PublisherConfig struct {
	// RegistryURL is the OCI registry URL, e.g., "oci://ghcr.io/myorg/charts"
	// or a classic HTTP repository URL.
	RegistryURL string
	// Username for registry authentication.
	Username string
	// Password or token for registry authentication.
	Password string
	// Timeout for HTTP operations (defaults to 30s).
	Timeout time.Duration
}

// NewPublisher creates a chart Publisher.
func NewPublisher(cfg PublisherConfig) *Publisher {
	t := cfg.Timeout
	if t == 0 {
		t = defaultTimeout
	}
	return &Publisher{cfg: cfg, client: &http.Client{Timeout: t}}
}

// Package runs "helm package" on the chart directory and returns the path to
// the resulting .tgz file.
func (p *Publisher) Package(ctx context.Context, chartDir, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "."
	}
	args := []string{"package", chartDir, "--destination", outputDir}
	cmd := exec.CommandContext(ctx, "helm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("helm: package: %w\n%s", err, out)
	}

	// Parse "Successfully packaged chart and saved it to: /path/chart-1.0.0.tgz"
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, ": "); idx != -1 {
		return strings.TrimSpace(line[idx+2:]), nil
	}
	return "", nil
}

// PushOCI pushes a packaged .tgz chart to an OCI registry using "helm push".
func (p *Publisher) PushOCI(ctx context.Context, chartTGZ, registry string) error {
	args := []string{"push", chartTGZ, registry}
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(),
		"HELM_REGISTRY_CONFIG=",
	)
	if p.cfg.Username != "" {
		cmd.Env = append(cmd.Env,
			"HELM_REGISTRY_CONFIG_USERNAME="+p.cfg.Username,
			"HELM_REGISTRY_CONFIG_PASSWORD="+p.cfg.Password,
		)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helm: push OCI: %w\n%s", err, out)
	}
	return nil
}

// UploadToHTTPRepo uploads a chart .tgz to a Helm HTTP chart repository
// using a simple PUT/POST request (compatible with ChartMuseum and similar).
func (p *Publisher) UploadToHTTPRepo(ctx context.Context, chartTGZ string) error {
	f, err := os.Open(chartTGZ)
	if err != nil {
		return fmt.Errorf("helm: open chart: %w", err)
	}
	defer f.Close()

	url := strings.TrimRight(p.cfg.RegistryURL, "/") + "/api/charts"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, f)
	if err != nil {
		return fmt.Errorf("helm: create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if p.cfg.Username != "" {
		req.SetBasicAuth(p.cfg.Username, p.cfg.Password)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("helm: upload chart: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("helm: chart version already exists (409 Conflict)")
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("helm: upload chart: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ReadChartMeta reads and parses a Chart.yaml file.
func ReadChartMeta(chartYAMLPath string) (*ChartMeta, error) {
	data, err := os.ReadFile(chartYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("helm: read Chart.yaml: %w", err)
	}
	var meta ChartMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("helm: parse Chart.yaml: %w", err)
	}
	return &meta, nil
}

// IsHelmAvailable reports whether the helm CLI is installed.
func IsHelmAvailable() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// ChartYAMLPath returns the path to Chart.yaml in a chart directory.
func ChartYAMLPath(chartDir string) string {
	return filepath.Join(chartDir, "Chart.yaml")
}

