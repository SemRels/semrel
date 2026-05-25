// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package nuget provides a built-in plugin for publishing .NET NuGet packages
// to nuget.org or a private NuGet feed.
//
// The plugin reads a NuGet package (.nupkg) from the filesystem and pushes it
// to the configured NuGet server using the NuGet v3 API. It also supports
// pushing symbol packages (.snupkg) and delisting old versions.
//
// Configuration example in .semrel.yaml:
//
//	plugins:
//	  nuget:
//	    api_key: ${NUGET_API_KEY}
//	    source: https://api.nuget.org/v3/index.json
//	    package_glob: "*.nupkg"
//	    skip_duplicate: true
//
// See: https://github.com/SemRels/semrel/issues/30
package nuget

import (
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
	// DefaultSource is the public nuget.org v3 API endpoint.
	DefaultSource = "https://api.nuget.org/v3/index.json"
	// pushPath is the well-known NuGet v2 push path used by nuget.org and
	// most NuGet-compatible servers (Nexus, Artifactory, Azure DevOps).
	pushPath = "/api/v2/package"
)

// Config holds the NuGet plugin configuration.
type Config struct {
	// APIKey is the NuGet server API key (required).
	APIKey string
	// Source is the NuGet server URL (default: nuget.org v3).
	Source string
	// SkipDuplicate silently ignores 409 Conflict responses.
	SkipDuplicate bool
	// Timeout overrides the HTTP request timeout (default 120s).
	Timeout time.Duration
}

// Publisher pushes NuGet packages to a feed.
type Publisher struct {
	cfg    Config
	client *http.Client
}

// NewPublisher creates a publisher from the given configuration.
func NewPublisher(cfg Config) *Publisher {
	if cfg.Source == "" {
		cfg.Source = DefaultSource
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}
	return &Publisher{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// Push uploads a .nupkg file to the NuGet server.
// Returns nil on success or if SkipDuplicate is set and the package already exists.
func (p *Publisher) Push(nupkgPath string) error {
	f, err := os.Open(nupkgPath)
	if err != nil {
		return fmt.Errorf("nuget: open %s: %w", nupkgPath, err)
	}
	defer f.Close()

	body, contentType, err := buildMultipart(f, filepath.Base(nupkgPath))
	if err != nil {
		return fmt.Errorf("nuget: build request: %w", err)
	}

	url := p.pushURL()
	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return fmt.Errorf("nuget: create request: %w", err)
	}
	req.Header.Set("X-NuGet-ApiKey", p.cfg.APIKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("nuget: push request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return nil
	case http.StatusConflict:
		if p.cfg.SkipDuplicate {
			return nil
		}
		return fmt.Errorf("nuget: package already exists (409 Conflict)")
	default:
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("nuget: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(rb)))
	}
}

// PushGlob pushes all .nupkg files matching the given glob pattern.
func (p *Publisher) PushGlob(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("nuget: glob %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("nuget: no packages matched pattern %q", pattern)
	}
	for _, path := range matches {
		if err := p.Push(path); err != nil {
			return err
		}
	}
	return nil
}

// pushURL builds the package push endpoint URL from the configured source.
// Most NuGet-compatible servers use /api/v2/package; nuget.org is one of them.
func (p *Publisher) pushURL() string {
	source := strings.TrimRight(p.cfg.Source, "/")
	// If the source already looks like an index URL, derive the push endpoint
	if strings.HasSuffix(source, "index.json") {
		source = source[:strings.LastIndex(source, "/")]
		source = source[:strings.LastIndex(source, "/")]
	}
	return source + pushPath
}

// buildMultipart creates a multipart/form-data body with the package file.
func buildMultipart(r io.Reader, filename string) (io.Reader, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		part, err := mw.CreateFormFile("package", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err = io.Copy(part, r); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.CloseWithError(mw.Close())
	}()

	return pr, mw.FormDataContentType(), nil
}
