// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package homebrew provides a built-in plugin for updating Homebrew tap formula
// files when a new release is published.
//
// A Homebrew tap formula is a Ruby file that describes how to install a tool.
// When a new version is released, the formula needs:
//  1. The URL updated to point to the new release tarball
//  2. The sha256 checksum updated to the new tarball's hash
//  3. The version string updated
//
// This package can update an existing formula file in-place, or generate a
// new formula from a template. The updated formula can then be committed to
// the tap repository.
//
// See: https://github.com/SemRels/semrel/issues/31
package homebrew

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// FormulaConfig holds the configuration for a Homebrew formula update.
type FormulaConfig struct {
	// Name is the formula name (e.g. "semrel").
	Name string
	// Version is the new version string (without leading 'v').
	Version string
	// URLTemplate is the download URL with {version} placeholder.
	// e.g. "https://github.com/SemRels/semrel/releases/download/v{version}/semrel_{version}_linux_amd64.tar.gz"
	URLTemplate string
	// SHA256 is the pre-computed checksum. If empty, it is downloaded and computed.
	SHA256 string
	// HTTPTimeout is the timeout for downloading the tarball (default 120s).
	HTTPTimeout time.Duration
}

// Updater updates Homebrew formula files.
type Updater struct {
	cfg    FormulaConfig
	client *http.Client
}

// NewUpdater creates an Updater from the given configuration.
func NewUpdater(cfg FormulaConfig) *Updater {
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 120 * time.Second
	}
	return &Updater{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

// DownloadURL returns the resolved download URL for the current version.
func (u *Updater) DownloadURL() string {
	return strings.ReplaceAll(u.cfg.URLTemplate, "{version}", u.cfg.Version)
}

// ComputeSHA256 downloads the tarball and returns its SHA-256 hex digest.
// If FormulaConfig.SHA256 is already set, it is returned directly.
func (u *Updater) ComputeSHA256() (string, error) {
	if u.cfg.SHA256 != "" {
		return u.cfg.SHA256, nil
	}
	url := u.DownloadURL()
	resp, err := u.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("homebrew: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("homebrew: download %s: HTTP %d", url, resp.StatusCode)
	}
	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return "", fmt.Errorf("homebrew: hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// UpdateFormulaFile reads the formula at path, replaces the url, sha256, and
// version fields, and writes the updated formula back to the same file.
func (u *Updater) UpdateFormulaFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("homebrew: read formula: %w", err)
	}

	checksum, err := u.ComputeSHA256()
	if err != nil {
		return err
	}

	updated := updateFormulaContent(string(data), u.DownloadURL(), checksum, u.cfg.Version)

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("homebrew: write formula: %w", err)
	}
	return nil
}

// GenerateFormula renders a complete Homebrew formula using the built-in
// template. Returns the formula content as a string.
func (u *Updater) GenerateFormula() (string, error) {
	checksum, err := u.ComputeSHA256()
	if err != nil {
		return "", err
	}
	data := formulaTemplateData{
		Name:    capitalize(u.cfg.Name),
		Version: u.cfg.Version,
		URL:     u.DownloadURL(),
		SHA256:  checksum,
	}
	var sb strings.Builder
	if err := formulaTemplate.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("homebrew: render template: %w", err)
	}
	return sb.String(), nil
}

type formulaTemplateData struct {
	Name, Version, URL, SHA256 string
}

var formulaTemplate = template.Must(template.New("formula").Funcs(template.FuncMap{
	"lower": strings.ToLower,
}).Parse(`class {{.Name}} < Formula
  desc "Semantic release automation"
  homepage "https://github.com/SemRels/semrel"
  url "{{.URL}}"
  sha256 "{{.SHA256}}"
  version "{{.Version}}"
  license "Apache-2.0"

  def install
    bin.install "{{lower .Name}}"
  end

  test do
    system "#{bin}/{{lower .Name}}", "--version"
  end
end
`))

var (
	reURL     = regexp.MustCompile(`(?m)^(\s*url\s+)"[^"]*"`)
	reSHA256  = regexp.MustCompile(`(?m)^(\s*sha256\s+)"[^"]*"`)
	reVersion = regexp.MustCompile(`(?m)^(\s*version\s+)"[^"]*"`)
)

func updateFormulaContent(content, url, sha256, version string) string {
	content = reURL.ReplaceAllString(content, `${1}"`+url+`"`)
	content = reSHA256.ReplaceAllString(content, `${1}"`+sha256+`"`)
	content = reVersion.ReplaceAllString(content, `${1}"`+version+`"`)
	return content
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
