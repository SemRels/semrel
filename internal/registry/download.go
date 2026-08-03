// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SemRels/semrel/pkg/semver"
)

const maxDownloadAttempts = 3

var pluginVersionPattern = regexp.MustCompile(
	`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?` +
		`(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
)

// DownloadPlugin resolves a plugin locally and downloads it from the registry when missing.
func (c *RegistryClient) DownloadPlugin(ctx context.Context, plugin, version, goos, goarch string) (string, error) {
	requestedCachePath, err := validatedPluginRefCachePath(plugin)
	if err != nil {
		return "", err
	}
	if version != "" {
		if !pluginVersionPattern.MatchString(version) {
			return "", newRegistryError(ErrCodeInvalidMetadata, fmt.Sprintf("invalid plugin version %q", version), nil)
		}
		if _, err := semver.ParseVersion(strings.TrimPrefix(version, "v")); err != nil {
			return "", newRegistryError(ErrCodeInvalidMetadata, fmt.Sprintf("invalid plugin version %q", version), err)
		}
	}

	// Preserve offline compatibility with cache entries written by older clients.
	if existingPath, err := findLocalPluginBinary(c.localPluginDir(requestedCachePath, version, goos, goarch)); err != nil {
		return "", err
	} else if existingPath != "" {
		return existingPath, nil
	}

	metadata, err := c.FetchMetadata(ctx)
	if err != nil {
		return "", err
	}

	pluginMeta, err := metadata.FindPlugin(plugin)
	if err != nil {
		return "", err
	}

	artifactName, err := pluginMeta.ValidatedExecutableName()
	if err != nil {
		return "", err
	}
	resolvedCachePath, err := validatedPluginRefCachePath(pluginMeta.CanonicalRef())
	if err != nil {
		return "", err
	}
	cacheNames := []string{resolvedCachePath}
	if pluginMeta.IsFirstParty() || !strings.HasPrefix(pluginMeta.CanonicalRef(), "@") {
		cacheNames = uniqueStrings(artifactName, pluginMeta.Name, requestedCachePath)
	}
	for _, cacheName := range cacheNames {
		localDir := c.localPluginDir(cacheName, version, goos, goarch)
		if existingPath, findErr := findLocalPluginBinary(localDir); findErr != nil {
			return "", findErr
		} else if existingPath != "" {
			return existingPath, nil
		}
	}

	localDir := c.localPluginDir(resolvedCachePath, version, goos, goarch)
	pluginVersion, err := pluginMeta.FindVersion(version)
	if err != nil {
		return "", err
	}

	checksum, err := pluginVersion.ChecksumForPlatform(goos, goarch)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return "", newRegistryError(ErrCodeCacheError, "create local plugin directory", err)
	}

	downloadURL := pluginVersion.DownloadURLForPlatform(goos, goarch)
	targetPath := filepath.Join(localDir, downloadFileName(downloadURL, artifactName, goos))
	if err := c.downloadWithRetry(ctx, downloadURL, targetPath, checksum); err != nil {
		return "", err
	}
	return targetPath, nil
}

func validatedPluginRefCachePath(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "@") {
		parts := strings.Split(ref, "/")
		scope := strings.TrimPrefix(parts[0], "@")
		if len(parts) != 2 || !artifactNamePattern.MatchString(scope) {
			return "", newRegistryError(ErrCodeInvalidMetadata, fmt.Sprintf("invalid plugin reference %q", ref), nil)
		}
		name, err := validatedArtifactName(parts[1])
		if err != nil {
			return "", err
		}
		return filepath.Join("@"+scope, name), nil
	}
	return validatedArtifactName(ref)
}

func validatedArtifactName(name string) (string, error) {
	meta := PluginMeta{ArtifactName: strings.TrimSpace(name)}
	return meta.ValidatedExecutableName()
}

func uniqueStrings(values ...string) []string {
	var result []string
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

// ValidateChecksum validates a file against the expected SHA-256 checksum.
func (c *RegistryClient) ValidateChecksum(filePath, expected string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return newRegistryError(ErrCodeCacheError, "open file for checksum validation", err)
	}
	defer file.Close() //nolint:errcheck

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return newRegistryError(ErrCodeCacheError, "calculate file checksum", err)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(strings.TrimSpace(expected), actual) {
		return newRegistryError(ErrCodeInvalidChecksum, fmt.Sprintf("checksum mismatch for %s", filePath), nil)
	}
	return nil
}

func (c *RegistryClient) downloadWithRetry(ctx context.Context, downloadURL, targetPath, checksum string) error {
	var lastErr error
	for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
		lastErr = c.downloadOnce(ctx, downloadURL, targetPath, checksum)
		if lastErr == nil {
			return nil
		}
		if !isRetryableDownloadError(lastErr) || attempt == maxDownloadAttempts {
			return lastErr
		}
		timer := time.NewTimer(time.Duration(attempt) * 200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return newRegistryError(ErrCodeNetworkError, "download cancelled", ctx.Err())
		case <-timer.C:
		}
	}
	return lastErr
}

func (c *RegistryClient) downloadOnce(ctx context.Context, downloadURL, targetPath, checksum string) error {
	requestCtx, cancel := context.WithTimeout(ctx, defaultDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return newRegistryError(ErrCodeNetworkError, "create plugin download request", err)
	}
	req.Header.Set("User-Agent", c.userAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return newRegistryError(ErrCodeNetworkError, "download plugin binary", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return newRegistryError(ErrCodeNotFound, fmt.Sprintf("plugin download not found at %s", downloadURL), nil)
	}
	if resp.StatusCode != http.StatusOK {
		return newRegistryError(ErrCodeNetworkError, fmt.Sprintf("plugin download failed with status %s", resp.Status), nil)
	}

	partialPath := targetPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return newRegistryError(ErrCodeCacheError, "remove stale partial download", err)
	}

	file, err := os.Create(partialPath)
	if err != nil {
		return newRegistryError(ErrCodeCacheError, "create partial plugin download", err)
	}

	copyErr := copyWithProgress(file, resp.Body, resp.ContentLength, path.Base(targetPath))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(partialPath)
		return newRegistryError(ErrCodeNetworkError, "write plugin download", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(partialPath)
		return newRegistryError(ErrCodeCacheError, "close partial plugin download", closeErr)
	}

	if err := c.ValidateChecksum(partialPath, checksum); err != nil {
		_ = os.Remove(partialPath)
		return err
	}

	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(partialPath)
		return newRegistryError(ErrCodeCacheError, "replace downloaded plugin", err)
	}
	if err := os.Rename(partialPath, targetPath); err != nil {
		_ = os.Remove(partialPath)
		return newRegistryError(ErrCodeCacheError, "finalize plugin download", err)
	}
	return nil
}

func findLocalPluginBinary(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", newRegistryError(ErrCodeCacheError, "read local plugin directory", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".partial") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return "", nil
	}

	sort.Strings(names)
	return filepath.Join(dir, names[0]), nil
}

func downloadFileName(downloadURL, plugin, goos string) string {
	parsed, err := url.Parse(downloadURL)
	if err == nil {
		base := path.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			return base
		}
	}

	if goos == "windows" {
		return plugin + ".exe"
	}
	return plugin
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, label string) error {
	buffer := make([]byte, 32*1024)
	var written int64
	lastReportedBucket := int64(-1)
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			written += int64(n)
			if total > 0 {
				bucket := (written * 10) / total
				if bucket > 10 {
					bucket = 10
				}
				if bucket != lastReportedBucket {
					lastReportedBucket = bucket
					log.Printf("downloading %s: %d%%", label, bucket*10)
				}
			}
		}
		if errors.Is(err, io.EOF) {
			if total <= 0 {
				log.Printf("downloaded %s: %d bytes", label, written)
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func isRetryableDownloadError(err error) bool {
	var registryErr *RegistryError
	if !errors.As(err, &registryErr) {
		return true
	}
	return registryErr.Code == ErrCodeNetworkError
}
