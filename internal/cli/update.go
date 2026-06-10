// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	buildversion "github.com/SemRels/semrel/internal/version"
)

const (
	githubReleasesAPI = "https://api.github.com/repos/SemRels/semrel/releases/latest"
	updateTimeout     = 5 * time.Minute
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func newUpdateCommand() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update semrel to the latest release",
		Long: `Check for a newer semrel release and install it.

Downloads the latest binary from https://github.com/SemRels/semrel/releases
and replaces the current executable in-place. A backup is kept as semrel.old
alongside the binary and is removed on the next successful run.

Examples:
  semrel update          — check and install the latest release
  semrel update --check  — check for updates without downloading

Exit codes:
  0 — already up to date, or update installed successfully
  1 — error checking or downloading the update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for a newer version, do not download")
	return cmd
}

func runUpdate(ctx context.Context, checkOnly bool) error {
	current := strings.TrimPrefix(buildversion.Version, "v")

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("fetching latest release info: %w", err)
	}
	latest := strings.TrimPrefix(release.TagName, "v")

	switch {
	case current == "dev":
		fmt.Printf("Current version: dev (built from source)\n")
		fmt.Printf("Latest release:  %s\n", release.TagName)
	case current == latest:
		fmt.Printf("✔  Already up to date (%s)\n", release.TagName)
		return nil
	default:
		fmt.Printf("Current version: v%s\n", current)
		fmt.Printf("Latest release:  %s\n", release.TagName)
	}

	if checkOnly {
		if current != latest {
			fmt.Printf("→  Run 'semrel update' to upgrade\n")
		}
		return nil
	}

	execPath, err := resolveExecutablePath()
	if err != nil {
		return err
	}

	fmt.Printf("Downloading %s for %s/%s…\n", release.TagName, runtime.GOOS, runtime.GOARCH)

	tmpDir, err := os.MkdirTemp("", "semrel-update-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	archiveURL := buildUpdateArchiveURL(release.TagName)
	archiveDest := filepath.Join(tmpDir, archiveBaseName(release.TagName))
	if err := downloadUpdateFile(ctx, archiveURL, archiveDest); err != nil {
		return fmt.Errorf("downloading %s: %w", archiveURL, err)
	}

	newBinary, err := extractSemrelBinary(archiveDest, tmpDir)
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	if err := swapBinary(newBinary, execPath); err != nil {
		return fmt.Errorf("installing update: %w", err)
	}

	fmt.Printf("✔  Updated to %s → %s\n", release.TagName, execPath)
	return nil
}

// fetchLatestRelease queries the GitHub Releases API for the latest semrel release.
func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, githubReleasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "semrel/"+buildversion.Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API responded with %s", resp.Status)
	}

	var r githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	if r.TagName == "" {
		return nil, fmt.Errorf("release tag is empty — unexpected response from GitHub API")
	}
	return &r, nil
}

func archiveBaseName(tag string) string {
	ver := strings.TrimPrefix(tag, "v")
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("semrel_%s_%s_%s.zip", ver, runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf("semrel_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
}

func buildUpdateArchiveURL(tag string) string {
	return fmt.Sprintf("https://github.com/SemRels/semrel/releases/download/%s/%s",
		tag, archiveBaseName(tag))
}

func downloadUpdateFile(ctx context.Context, url, dest string) error {
	reqCtx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "semrel/"+buildversion.Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no release asset found at %s — platform %s/%s may not be supported yet",
			url, runtime.GOOS, runtime.GOARCH)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	_, err = io.Copy(f, resp.Body) //nolint:gosec
	return err
}

// extractSemrelBinary extracts the semrel binary from a .tar.gz or .zip archive.
func extractSemrelBinary(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, destDir)
	}
	return extractFromTarGz(archivePath, destDir)
}

func extractFromTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close() //nolint:errcheck

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) == "semrel" {
			outPath := filepath.Join(destDir, "semrel")
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(out, tr) //nolint:gosec
			closeErr := out.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			return outPath, nil
		}
	}
	return "", fmt.Errorf("semrel binary not found inside archive")
}

func extractFromZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close() //nolint:errcheck

	for _, f := range r.File {
		if filepath.Base(f.Name) == "semrel.exe" {
			outPath := filepath.Join(destDir, "semrel.exe")
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				rc.Close()
				return "", err
			}
			_, copyErr := io.Copy(out, rc) //nolint:gosec
			rc.Close()
			closeErr := out.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			return outPath, nil
		}
	}
	return "", fmt.Errorf("semrel.exe not found inside archive")
}

// swapBinary atomically replaces targetPath with newBinary.
// On Windows a running exe cannot be overwritten, so we rename it to .old first.
// After the swap, the Zone.Identifier alternate data stream is removed so that
// Smart App Control / Windows Defender does not block the new binary.
func swapBinary(newBinary, targetPath string) error {
	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(targetPath, oldPath); err != nil {
		return fmt.Errorf("backing up current binary to %s: %w", oldPath, err)
	}

	if err := os.Rename(newBinary, targetPath); err != nil {
		_ = os.Rename(oldPath, targetPath) // restore
		return fmt.Errorf("moving new binary into place: %w", err)
	}

	_ = os.Remove(oldPath) // best-effort; may fail on Windows while process is still live
	unblockWindowsBinary(targetPath)
	return nil
}

// unblockWindowsBinary removes the Zone.Identifier alternate data stream on Windows
// so that Smart App Control does not block the newly placed binary.
// This is a no-op on non-Windows platforms.
func unblockWindowsBinary(path string) {
	if runtime.GOOS != "windows" {
		return
	}
	// Remove the Zone.Identifier ADS that marks the file as downloaded from the internet.
	_ = os.Remove(path + ":Zone.Identifier")
}

func resolveExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding current executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks for %s: %w", execPath, err)
	}
	return resolved, nil
}
