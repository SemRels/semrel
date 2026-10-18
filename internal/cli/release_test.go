// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SemRels/semrel/internal/colors"
	"github.com/SemRels/semrel/internal/registry"
)

var workingDirMu sync.Mutex

func TestRunReleaseLoadConfigError(t *testing.T) {
	withColorsDisabled(t)
	if err := runRelease(context.Background(), false, "missing.yaml", false, false, false, "text", false, "", "", ""); err == nil || !strings.Contains(err.Error(), "loading config") {
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
		return runRelease(context.Background(), true, "", false, false, false, "json", false, "", "", "")
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
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: develop\ntagPrefix: \"v\"\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "json", false, "", "", "")
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
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"v\"\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "json", false, "", "", "")
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
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"v\"\n")

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, ".semrel.yaml", true, false, false, "text", false, "", "", "")
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
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"v\"\nplugins:\n  - uses: definitely-not-installed\n")

	// Use a fast-failing registry URL so auto-install does not make real network calls.
	t.Setenv(registry.EnvRegistryURL, "http://127.0.0.1:0")
	t.Setenv(registry.EnvCacheDir, t.TempDir())

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "text", false, "", "", "")
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
	// With auto-install enabled the runner prints a "not found" message and
	// gracefully skips the unavailable plugin.
	if !strings.Contains(stdout, `plugin "definitely-not-installed" not found`) {
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

// This code has been partially generated by an AI.
func TestRunReleaseUsesConfiguredBumpForVersion(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v1.0.0")
	runReleaseGit(t, repoDir, "commit", "--allow-empty", "-m", "chore: trigger configured patch release")
	writeReleaseConfig(t, repoDir, `schemaVersion: 1
branches:
  - name: main
tagPrefix: "v"
rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: perf
    bump: patch
  - type: revert
    bump: patch
  - type: chore
    bump: patch
`)

	withWorkingDir(t, repoDir)
	stdout, stderr, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), false, ".semrel.yaml", false, false, false, "json", false, "", "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v\nstderr: %s", err, stderr)
	}

	summary := decodeReleaseSummary(t, stdout)
	if !summary.Released {
		t.Fatal("expected the configured chore patch rule to create a release")
	}
	if summary.Bump != "patch" {
		t.Fatalf("summary.Bump = %q, want patch", summary.Bump)
	}
	if summary.NextVersion != "v1.0.1" {
		t.Fatalf("summary.NextVersion = %q, want v1.0.1", summary.NextVersion)
	}
	if tags := runReleaseGit(t, repoDir, "tag", "--list"); !strings.Contains(tags, "v1.0.1") {
		t.Fatalf("expected release tag v1.0.1, got %q", tags)
	}
}

func TestRunReleaseDryRunPluginFailureIsNonFatal(t *testing.T) {
	// Verify that a plugin that exits non-zero during --dry-run only prints a
	// warning to stderr and does NOT cause runRelease to return an error.
	// Real-world case: provider-gitlab fails with "SEMREL_PLUGIN_TOKEN is required"
	// when the token is not available in a local dry-run environment.
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v0.1.0")
	commitReleaseFile(t, repoDir, "feature.txt", "new\n", "feat: dry-run test")

	// Write a config that uses a failing plugin (path-based so no registry call needed).
	// The script exits 1 to simulate a missing-token failure.
	failPlugin := filepath.Join(t.TempDir(), "semrel-plugin-fail")
	if err := os.WriteFile(failPlugin, []byte("#!/bin/sh\necho 'SEMREL_PLUGIN_TOKEN is required' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"v\"\nplugins:\n  - path: "+failPlugin+"\n")

	t.Setenv(registry.EnvRegistryURL, "http://127.0.0.1:0")
	t.Setenv(registry.EnvCacheDir, t.TempDir())
	withWorkingDir(t, repoDir)

	_, _, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, ".semrel.yaml", false, false, false, "text", false, "", "", "")
	})
	// Dry-run must succeed even when a plugin fails.
	if err != nil {
		t.Fatalf("dry-run should not fail on plugin error, got: %v", err)
	}
	// No tag must have been created.
	tags := runReleaseGit(t, repoDir, "tag", "--list")
	if strings.Contains(tags, "v0.2.0") {
		t.Fatal("dry-run must not create a tag")
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

func TestRunReleasePrereleaseChannel(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v1.2.0")
	commitReleaseFile(t, repoDir, "feature.txt", "new\n", "feat: add next feature")
	// Configure a pre-release branch: current branch is "main", prerelease: "next"
	writeReleaseConfig(t, repoDir, `schemaVersion: 1
branches:
  - name: main
    prerelease: next
tagPrefix: "v"
`)
	t.Setenv(registry.EnvRegistryURL, "http://127.0.0.1:0")
	t.Setenv(registry.EnvCacheDir, t.TempDir())
	withWorkingDir(t, repoDir)

	stdout, _, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, ".semrel.yaml", false, false, false, "text", false, "", "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	// Must produce a pre-release version like v1.3.0-next.1, not v1.3.0
	if !strings.Contains(stdout, "next.") {
		t.Fatalf("expected pre-release version with 'next.', stdout = %q", stdout)
	}
	if strings.Contains(stdout, "v1.3.0\n") {
		t.Fatalf("must not produce stable version on pre-release branch, stdout = %q", stdout)
	}
}

func TestRunReleaseNextVersionOverride(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "README.md", "hello\n", "feat: initial")
	runReleaseGit(t, repoDir, "tag", "v1.0.0")
	commitReleaseFile(t, repoDir, "feature.txt", "new\n", "feat: add feature")
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"v\"\n")
	t.Setenv(registry.EnvRegistryURL, "http://127.0.0.1:0")
	t.Setenv(registry.EnvCacheDir, t.TempDir())
	withWorkingDir(t, repoDir)

	stdout, _, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, ".semrel.yaml", false, false, false, "text", false, "", "", "3.0.0")
	})
	if err != nil {
		t.Fatalf("runRelease(--next-version 3.0.0) error = %v", err)
	}
	if !strings.Contains(stdout, "3.0.0") {
		t.Fatalf("expected overridden version 3.0.0, stdout = %q", stdout)
	}
}

func TestRunReleaseScopesCommitsToPackagePaths(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "pkg-a/a.txt", "a\n", "feat: bootstrap a")
	commitReleaseFile(t, repoDir, "pkg-b/b.txt", "b\n", "feat: bootstrap b")
	runReleaseGit(t, repoDir, "tag", "v1.0.0")
	commitReleaseFile(t, repoDir, "pkg-a/a.txt", "a2\n", "feat(a): new a feature")
	commitReleaseFile(t, repoDir, "pkg-b/b.txt", "b2\n", "fix(b): fix b")
	commitReleaseFile(t, repoDir, ".github/ci.yml", "ci\n", "feat: repo-level change")
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"v\"\n")

	run := func(t *testing.T, cwd string, paths ...string) ReleaseSummary {
		t.Helper()
		withWorkingDir(t, repoDir)
		ctx := withCommitPaths(context.Background(), paths...)
		stdout, _, err := captureReleaseOutput(func() error {
			err := runRelease(ctx, true, ".semrel.yaml", false, false, false, "json", false, "", "", "")
			if errors.Is(err, ErrNothingToRelease) {
				return nil
			}
			return err
		})
		if err != nil {
			t.Fatalf("runRelease() error = %v", err)
		}
		return decodeReleaseSummary(t, stdout[strings.Index(stdout, "{"):])
	}

	t.Run("package a sees only its own feat", func(t *testing.T) {
		s := run(t, repoDir, filepath.Join(repoDir, "pkg-a"))
		if s.Commits != 1 || s.Bump != "minor" {
			t.Fatalf("pkg-a: commits=%d bump=%q, want 1/minor (%v)", s.Commits, s.Bump, s.CommitMessages)
		}
	})
	t.Run("package b only gets a patch", func(t *testing.T) {
		s := run(t, repoDir, "pkg-b")
		if s.Commits != 1 || s.Bump != "patch" {
			t.Fatalf("pkg-b: commits=%d bump=%q, want 1/patch (%v)", s.Commits, s.Bump, s.CommitMessages)
		}
		if strings.Contains(s.Changelog, "new a feature") || strings.Contains(s.Changelog, "repo-level") {
			t.Fatalf("pkg-b changelog contains foreign commits:\n%s", s.Changelog)
		}
	})
	t.Run("package without changes releases nothing", func(t *testing.T) {
		if err := os.MkdirAll(filepath.Join(repoDir, "pkg-c"), 0o755); err != nil {
			t.Fatal(err)
		}
		s := run(t, repoDir, filepath.Join(repoDir, "pkg-c"))
		if s.Released || s.Commits != 0 || s.Bump != "" {
			t.Fatalf("pkg-c: %+v, want no release", s)
		}
	})
	t.Run("no scope sees every commit", func(t *testing.T) {
		s := run(t, repoDir)
		if s.Commits != 3 {
			t.Fatalf("unscoped commits = %d, want 3", s.Commits)
		}
	})
}

func TestRunReleaseUsesLastTagMatchingTagPrefix(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "aws-a/a.txt", "a\n", "feat: bootstrap a")
	runReleaseGit(t, repoDir, "tag", "-a", "aws-a-v1.2.0", "-m", "x")
	commitReleaseFile(t, repoDir, "aws-b/b.txt", "b\n", "feat: bootstrap b")
	runReleaseGit(t, repoDir, "tag", "-a", "aws-b-v5.0.0", "-m", "x")
	commitReleaseFile(t, repoDir, "aws-a/a.txt", "a2\n", "fix: fix a")
	writeReleaseConfig(t, repoDir, "schemaVersion: 1\nbranches:\n  - name: main\ntagPrefix: \"aws-a-v\"\n")

	withWorkingDir(t, repoDir)
	stdout, _, err := captureReleaseOutput(func() error {
		return runRelease(context.Background(), true, ".semrel.yaml", false, false, false, "json", false, "", "", "")
	})
	if err != nil {
		t.Fatalf("runRelease() error = %v", err)
	}
	s := decodeReleaseSummary(t, stdout[strings.Index(stdout, "{"):])
	if s.CurrentVersion != "aws-a-v1.2.0" || s.NextVersion != "aws-a-v1.3.0" {
		t.Fatalf("current=%q next=%q, want aws-a-v1.2.0 -> aws-a-v1.3.0", s.CurrentVersion, s.NextVersion)
	}
	// The range starts at aws-a-v1.2.0 (not at aws-b-v5.0.0), so b's bootstrap
	// feat is still included here; path scoping (#319) narrows it further.
	if s.Commits != 2 {
		t.Fatalf("commits = %d, want 2 (%v)", s.Commits, s.CommitMessages)
	}
}

func TestWorkspaceReleaseInheritsRootConfigAndTagPrefix(t *testing.T) {
	withColorsDisabled(t)
	repoDir := initReleaseRepo(t)
	commitReleaseFile(t, repoDir, "aws-a/a.txt", "a\n", "feat: bootstrap a")
	commitReleaseFile(t, repoDir, "aws-b/b.txt", "b\n", "feat: bootstrap b")
	runReleaseGit(t, repoDir, "tag", "-a", "aws-a-v1.0.0", "-m", "x")
	runReleaseGit(t, repoDir, "tag", "-a", "aws-b-v1.0.0", "-m", "x")
	commitReleaseFile(t, repoDir, "aws-a/a.txt", "a2\n", "fix: fix a")
	writeReleaseConfig(t, repoDir, `schemaVersion: 1
branches:
  - name: main
workspace:
  strategy: independent
  packages:
    - path: aws-a
      tagPrefix: "aws-a-v"
    - path: aws-b
      tagPrefix: "aws-b-v"
`)
	// aws-b has its own config that only overrides the branch list.
	if err := os.WriteFile(filepath.Join(repoDir, "aws-b", ".semrel.yaml"),
		[]byte("branches:\n  - name: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Relative, auto-detected root config: the failure mode from #321.
	withWorkingDir(t, repoDir)
	_, cfgs, err := resolveWorkspacePackages(context.Background(), "")
	if err != nil {
		t.Fatalf("resolveWorkspacePackages() error = %v", err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("packages = %d, want 2", len(cfgs))
	}
	for _, p := range cfgs {
		if !filepath.IsAbs(p.Path) || !filepath.IsAbs(p.RootConfig) {
			t.Fatalf("package paths must be absolute: %+v", p)
		}
	}

	dry := func(p workspacePkg) (ReleaseSummary, error) {
		if err := os.Chdir(p.Path); err != nil {
			t.Fatal(err)
		}
		var s ReleaseSummary
		stdout, _, err := captureReleaseOutput(func() error {
			return runRelease(p.context(context.Background()), true, p.ConfigFile, false, false, false, "json", false, "", "", "")
		})
		if err == nil || errors.Is(err, ErrNothingToRelease) {
			s = decodeReleaseSummary(t, stdout[strings.Index(stdout, "{"):])
			err = nil
		}
		return s, err
	}

	// aws-a has no config of its own: it inherits the root config (branch main)
	// and gets its tag prefix from the workspace entry.
	a, err := dry(cfgs[0])
	if err != nil {
		t.Fatalf("aws-a release error = %v", err)
	}
	if a.TagPrefix != "aws-a-v" || a.CurrentVersion != "aws-a-v1.0.0" || a.NextVersion != "aws-a-v1.0.1" {
		t.Fatalf("aws-a summary = %+v", a)
	}

	// aws-b overrides the branches, so main is not a release branch for it,
	// while the tag prefix still comes from the workspace entry.
	b, err := dry(cfgs[1])
	if err != nil {
		t.Fatalf("aws-b release error = %v", err)
	}
	if b.Released || b.TagPrefix != "aws-b-v" {
		t.Fatalf("aws-b summary = %+v, want skipped with prefix aws-b-v", b)
	}
}
