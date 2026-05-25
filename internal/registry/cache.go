// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The go-semrel Authors

package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	registryCacheDirName = "registry-cache"
	metadataFileName     = "plugins.json"
	metadataETagFileName = "plugins.json.etag"
	defaultCacheTTL      = 24 * time.Hour
)

func normalizeCacheDir(cacheDir string) string {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		cacheDir = ".semrel"
	}

	cleaned := filepath.Clean(cacheDir)
	if filepath.Base(cleaned) == registryCacheDirName {
		return cleaned
	}
	return filepath.Join(cleaned, registryCacheDirName)
}

func (c *RegistryClient) ensureCacheDir() error {
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return newRegistryError(ErrCodeCacheError, "create registry cache directory", err)
	}
	return nil
}

func (c *RegistryClient) metadataCachePath() string {
	return filepath.Join(c.cacheDir, metadataFileName)
}

func (c *RegistryClient) metadataETagPath() string {
	return filepath.Join(c.cacheDir, metadataETagFileName)
}

func (c *RegistryClient) cacheFresh() bool {
	info, err := os.Stat(c.metadataCachePath())
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < c.cacheTTL
}

func (c *RegistryClient) readCachedMetadata() (*PluginRegistry, string, error) {
	data, err := os.ReadFile(c.metadataCachePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", newRegistryError(ErrCodeCacheError, "read cached registry metadata", err)
	}

	var registry PluginRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, "", newRegistryError(ErrCodeInvalidMetadata, "parse cached registry metadata", err)
	}

	etagData, err := os.ReadFile(c.metadataETagPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", newRegistryError(ErrCodeCacheError, "read cached registry ETag", err)
	}

	return &registry, strings.TrimSpace(string(etagData)), nil
}

func (c *RegistryClient) writeCachedMetadata(data []byte, etag string) (*PluginRegistry, error) {
	var registry PluginRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, newRegistryError(ErrCodeInvalidMetadata, "parse registry metadata", err)
	}

	if err := c.ensureCacheDir(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(c.metadataCachePath(), data, 0o644); err != nil {
		return nil, newRegistryError(ErrCodeCacheError, "write cached registry metadata", err)
	}

	if etag == "" {
		if err := os.Remove(c.metadataETagPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, newRegistryError(ErrCodeCacheError, "remove cached registry ETag", err)
		}
	} else if err := os.WriteFile(c.metadataETagPath(), []byte(strings.TrimSpace(etag)), 0o644); err != nil {
		return nil, newRegistryError(ErrCodeCacheError, "write cached registry ETag", err)
	}

	return &registry, nil
}

func (c *RegistryClient) touchMetadataCache() error {
	now := time.Now()
	if err := os.Chtimes(c.metadataCachePath(), now, now); err != nil {
		return newRegistryError(ErrCodeCacheError, "refresh registry cache timestamp", err)
	}
	return nil
}

func (c *RegistryClient) semrelRootDir() string {
	if filepath.Base(c.cacheDir) == registryCacheDirName {
		return filepath.Dir(c.cacheDir)
	}
	return c.cacheDir
}

func (c *RegistryClient) localPluginDir(plugin, version, goos, goarch string) string {
	return filepath.Join(c.semrelRootDir(), platformKey(goos, goarch), plugin, version)
}

// ClearCache removes the local registry cache directory.
func (c *RegistryClient) ClearCache() error {
	if err := os.RemoveAll(c.cacheDir); err != nil {
		return newRegistryError(ErrCodeCacheError, "clear registry cache", err)
	}
	return nil
}
