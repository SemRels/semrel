// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	buildversion "github.com/GoSemantics/semrel/internal/version"
)

const (
	// DefaultBaseURL is the MVP registry location served through GitHub Pages.
	DefaultBaseURL = "https://semrels.github.io/semrel-registry"

	// EnvRegistryURL overrides the registry base URL.
	EnvRegistryURL = "SEMREL_REGISTRY_URL"
	// EnvCacheDir overrides the .semrel cache root directory.
	EnvCacheDir = "SEMREL_CACHE_DIR"
	// EnvCacheTTL overrides the registry metadata cache TTL.
	EnvCacheTTL = "SEMREL_CACHE_TTL"
)

const (
	defaultMetadataTimeout = 30 * time.Second
	defaultDownloadTimeout = 5 * time.Minute
)

// RegistryClient downloads and caches plugin registry metadata and binaries.
type RegistryClient struct {
	baseURL    string
	cacheDir   string
	cacheTTL   time.Duration
	httpClient *http.Client
}

// NewRegistryClient creates a registry client for the given base URL and cache root.
func NewRegistryClient(baseURL, cacheDir string) (*RegistryClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	client := &RegistryClient{
		baseURL:  baseURL,
		cacheDir: normalizeCacheDir(cacheDir),
		cacheTTL: defaultCacheTTL,
		httpClient: &http.Client{
			Timeout: defaultDownloadTimeout,
		},
	}

	if err := client.ensureCacheDir(); err != nil {
		return nil, err
	}
	return client, nil
}

// NewRegistryClientFromEnv creates a registry client from environment variables.
func NewRegistryClientFromEnv() (*RegistryClient, error) {
	baseURL := os.Getenv(EnvRegistryURL)
	cacheDir := os.Getenv(EnvCacheDir)
	client, err := NewRegistryClient(baseURL, cacheDir)
	if err != nil {
		return nil, err
	}

	if ttlValue := strings.TrimSpace(os.Getenv(EnvCacheTTL)); ttlValue != "" {
		ttl, err := time.ParseDuration(ttlValue)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", EnvCacheTTL, err)
		}
		client.cacheTTL = ttl
	}

	return client, nil
}

// FetchMetadata returns registry metadata using a 24h cache with conditional requests.
func (c *RegistryClient) FetchMetadata(ctx context.Context) (*PluginRegistry, error) {
	cachedRegistry, cachedETag, cacheErr := c.readCachedMetadata()
	if cacheErr != nil {
		log.Printf("warning: ignoring registry cache: %v", cacheErr)
		cachedETag = ""
	}
	if cachedRegistry != nil && c.cacheFresh() {
		return cachedRegistry, nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, defaultMetadataTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, c.metadataURL(), nil)
	if err != nil {
		return nil, newRegistryError(ErrCodeNetworkError, "create registry metadata request", err)
	}
	req.Header.Set("User-Agent", c.userAgent())
	if cachedETag != "" {
		req.Header.Set("If-None-Match", cachedETag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if cachedRegistry != nil {
			log.Printf("warning: failed to refresh registry metadata, using cached copy: %v", err)
			return cachedRegistry, nil
		}
		return nil, newRegistryError(ErrCodeNetworkError, "fetch registry metadata", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, newRegistryError(ErrCodeNetworkError, "read registry metadata response", err)
		}
		return c.writeCachedMetadata(data, resp.Header.Get("ETag"))
	case http.StatusNotModified:
		if cachedRegistry == nil {
			return nil, newRegistryError(ErrCodeCacheError, "registry returned 304 without cached metadata", nil)
		}
		if err := c.touchMetadataCache(); err != nil {
			return nil, err
		}
		return cachedRegistry, nil
	case http.StatusNotFound:
		return nil, newRegistryError(ErrCodeNotFound, "registry metadata not found", nil)
	default:
		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, newRegistryError(ErrCodeNetworkError, fmt.Sprintf("registry metadata request failed with status %s", resp.Status), nil)
		}
		return nil, newRegistryError(ErrCodeInvalidMetadata, fmt.Sprintf("unexpected registry metadata response status %s", resp.Status), nil)
	}
}

// GetPlugin returns a local path for the requested plugin, downloading it if necessary.
func (c *RegistryClient) GetPlugin(ctx context.Context, name, version, goos, goarch string) (string, error) {
	return c.DownloadPlugin(ctx, name, version, goos, goarch)
}

func (c *RegistryClient) metadataURL() string {
	if strings.HasSuffix(strings.ToLower(c.baseURL), metadataFileName) {
		return c.baseURL
	}
	return c.baseURL + "/" + metadataFileName
}

func (c *RegistryClient) userAgent() string {
	return "go-semrel/" + buildversion.Version
}
