// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/GoSemantics/semrel/internal/colors"
)

var workingDirMu sync.Mutex

func TestRunReleaseLoadConfigError(t *testing.T) {
	withColorsDisabled(t)
	if err := runRelease(context.Background(), false, "missing.yaml", false, false, false, "text", false, "", ""); err == nil || !strings.Contains(err.Error(), "loading config") {
		t.Fatalf("runRelease() error = %v", err)
	}
}

func TestRunReleaseAutoDetectsTOMLConfig(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	if err := os.WriteFile(filepath.Join(repoDir, ".semrel.toml"), []byte("tag_prefix = \"v\"\n\n[[branches]]\nname = \"main\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.semrel.toml) error = %v", err)
	}

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, "", false, false, false, "json", false, "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	if !strings.Contains(stdout, `"branch": "main"`) {
		t.Fatalf("stdout = %q, want JSON summary for main branch", stdout)
	}
}

func TestRunReleaseSkipsUnconfiguredBranchInJSON(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "docs: initial")
	writeReleaseConfig(t, repoDir, "branches:\n  - name: develop\ntagPrefix: \"v\"\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "json", false, "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	summary := decodeReleaseSummary(t, stdout)
	if summary.Released {
		t.Fatal("expected release to be skipped")
	}
	if summary.Branch != "main" {
		t.Fatalf("summary.Branch = %q, want main", summary.Branch)
	}
}

func TestRunReleaseNoCommitsSinceLastReleaseJSON(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v1.0.0")
	writeReleaseConfig(t, repoDir, "branches:\n  - name: main\ntagPrefix: \"v\"\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "json", false, "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	summary := decodeReleaseSummary(t, stdout)
	if summary.Released {
		t.Fatal("expected no release when there are no new commits")
	}
	if summary.CurrentVersion != "v1.0.0" {
		t.Fatalf("summary.CurrentVersion = %q, want v1.0.0", summary.CurrentVersion)
	}
}

func TestRunReleaseDryRunForcePatch(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v1.0.0")
	writeReleaseConfig(t, repoDir, "branches:\n  - name: main\ntagPrefix: \"v\"\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, ".semrel.yaml", true, false, false, "text", false, "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Would perform") || !strings.Contains(stdout, "git tag v1.0.1") {
		t.Fatalf("stdout = %q", stdout)
	}

	tags := runReleaseGit(t, repoDir, "tag", "--list")
	if strings.Contains(tags, "v1.0.1") {
		t.Fatalf("unexpected dry-run tag creation: %q", tags)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Fatalf("CHANGELOG.md should not be created during dry run, err=%v", err)
	}
}

func TestRunReleaseCreatesTagChangelogAndRunsPlugins(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v0.1.0")
	commitReleaseFile(t, repoDir, "feature.txt", "new feature\n", "feat: add release flow")
	writeReleaseConfig(t, repoDir, "branches:\n  - name: main\ntagPrefix: \"v\"\nplugins:\n  - uses: definitely-not-installed\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "text", false, "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Created tag v0.2.0") {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stdout, "plugin \"definitely-not-installed\" not installed") {
		t.Fatalf("stdout = %q", stdout)
	}

	tags := runReleaseGit(t, repoDir, "tag", "--list")
	if !strings.Contains(tags, "v0.2.0") {
		t.Fatalf("expected created tag in %q", tags)
	}
	changelog, err := os.ReadFile(filepath.Join(repoDir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("ReadFile(CHANGELOG.md) error = %v", err)
	}
	if !strings.Contains(string(changelog), "## v0.2.0") {
		t.Fatalf("unexpected changelog contents: %s", string(changelog))
	}
}

func initReleaseRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runReleaseGit(t, dir, "init")
	runReleaseGit(t, dir, "config", "user.email", "semrel@example.com")
	runReleaseGit(t, dir, "config", "user.name", "semrel tests")
	runReleaseGit(t, dir, "config", "commit.gpgsign", "false")
	runReleaseGit(t, dir, "config", "tag.gpgSign", "false")
	return dir
}

func commitReleaseFile(t *testing.T, dir, name, contents, message string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runReleaseGit(t, dir, "add", name)
	runReleaseGit(t, dir, "commit", "-m", message)
	runReleaseGit(t, dir, "branch", "-M", "main")
}

func writeReleaseConfig(t *testing.T, dir, body string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, ".semrel.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(.semrel.yaml) error = %v", err)
	}
}

func runReleaseGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()

	workingDirMu.Lock()
	oldDir, err := os.Getwd()
	if err != nil {
		workingDirMu.Unlock()
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		workingDirMu.Unlock()
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
		workingDirMu.Unlock()
	})
}

func withColorsDisabled(t *testing.T) {
	t.Helper()

	wasEnabled := colors.IsEnabled()
	colors.Disable()
	t.Cleanup(func() {
		if wasEnabled {
			colors.Enable()
		} else {
			colors.Disable()
		}
	})
}

func captureReleaseOutput(fn func() error) (string, string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		return "", "", err
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	runErr := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, _ = stdout.ReadFrom(stdoutReader)
	_, _ = stderr.ReadFrom(stderrReader)
	return stdout.String(), stderr.String(), runErr
}

func decodeReleaseSummary(t *testing.T, raw string) ReleaseSummary {
	t.Helper()

	var summary ReleaseSummary
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &summary); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", raw, err)
	}
	return summary
}
