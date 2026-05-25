// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Gitea/Forgejo Go Package Registry support.
//
// Gitea and Forgejo both implement a Go module proxy / package registry that
// allows private Go modules to be distributed via their instance. After a
// release, semrel can publish the Go module to the Gitea Go Package Registry
// so that it is available via GOPROXY.
//
// Publishing works by uploading the .zip archive (module source) and the
// .mod file to the Gitea package API endpoint.
//
// API reference:
// https://gitea.io/api/swagger#/packages
//
// See: https://github.com/SemRels/semrel/issues/97
package gitea

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// GoRegistryConfig holds configuration for the Gitea Go Package Registry.
type GoRegistryConfig struct {
	// BaseURL is the Gitea/Forgejo instance base URL (e.g. "https://gitea.example.com").
	BaseURL string
	// Token is the Gitea access token with package:write permission.
	Token string
	// Owner is the Gitea user or organization that owns the package.
	Owner string
	// ModulePath is the Go module path (e.g. "github.com/myorg/myapp").
	ModulePath string
	// Version is the version to publish (without 'v' prefix; e.g. "1.2.3").
	Version string
}

// GoRegistryPublisher publishes Go modules to a Gitea/Forgejo package registry.
type GoRegistryPublisher struct {
	cfg    GoRegistryConfig
	client *http.Client
}

// NewGoRegistryPublisher creates a publisher from the given configuration.
func NewGoRegistryPublisher(cfg GoRegistryConfig) *GoRegistryPublisher {
	return &GoRegistryPublisher{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// PublishModule uploads a Go module archive (.zip) and go.mod file to the
// Gitea package registry for the configured owner/module/version.
//
// The zipContent is the module source zip (as produced by `go mod download`
// or `go mod zip`). The modContent is the go.mod file bytes.
func (p *GoRegistryPublisher) PublishModule(zipContent []byte, modContent []byte) error {
	if err := p.uploadFile(zipContent, p.zipFilename(), "application/zip"); err != nil {
		return fmt.Errorf("gitea go-registry: upload zip: %w", err)
	}
	if err := p.uploadFile(modContent, p.modFilename(), "text/plain"); err != nil {
		return fmt.Errorf("gitea go-registry: upload mod: %w", err)
	}
	return nil
}

// PackageURL returns the URL of the published module in the registry.
func (p *GoRegistryPublisher) PackageURL() string {
	return fmt.Sprintf("%s/%s/-/packages/go/%s/%s",
		strings.TrimRight(p.cfg.BaseURL, "/"),
		p.cfg.Owner,
		escapePath(p.cfg.ModulePath),
		"v"+p.cfg.Version,
	)
}

func (p *GoRegistryPublisher) uploadFile(content []byte, filename, contentType string) error {
	apiURL := fmt.Sprintf("%s/api/packages/%s/go/upload",
		strings.TrimRight(p.cfg.BaseURL, "/"),
		p.cfg.Owner,
	)

	body, ct, err := buildGoRegistryForm(content, filename)
	if err != nil {
		return err
	}
	_ = contentType // multipart determines content type

	req, err := http.NewRequest(http.MethodPut, apiURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+p.cfg.Token)
	req.Header.Set("Content-Type", ct)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusConflict:
		return fmt.Errorf("version already exists in registry")
	default:
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(rb)))
	}
}

func (p *GoRegistryPublisher) zipFilename() string {
	// Go module zip format: <module>@v<version>.zip
	return fmt.Sprintf("%s@v%s.zip", escapePath(p.cfg.ModulePath), p.cfg.Version)
}

func (p *GoRegistryPublisher) modFilename() string {
	return fmt.Sprintf("%s@v%s.mod", escapePath(p.cfg.ModulePath), p.cfg.Version)
}

// escapePath replaces "/" with "!" for use in package filenames (Gitea convention).
func escapePath(module string) string {
	// Gitea uses the module path as-is in the URL; for filenames replace / with !
	return strings.ReplaceAll(module, "/", "!")
}

func buildGoRegistryForm(content []byte, filename string) (io.Reader, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err = io.Copy(part, bytes.NewReader(content)); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.CloseWithError(mw.Close())
	}()
	return pr, mw.FormDataContentType(), nil
}
