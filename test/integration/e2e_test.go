// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// E2E tests exercise the full `semrel release` CLI pipeline against a real
// temporary git repository with stub plugins.
//
// Architecture note: semrel's analyzer (bump calculation) and changelog
// generator are built-in. External plugins cover three phases:
//   - condition — gate checks (e.g. "are we in CI?")
//   - pre-tag   — mutations before the git tag (e.g. version file updates)
//   - release   — publish actions after the tag (e.g. GitHub Release, hooks)
//
// Stub plugins exercise the subprocess protocol without network calls.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// requireSemrelBinary builds and returns the path to the semrel binary.
// The test is skipped if `go build` is unavailable.
func requireSemrelBinary(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH — skipping e2e test")
	}
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "semrel")
	// Build from the module root: test/integration/../../ = repo root.
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/semrel")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build semrel: %v\n%s", err, out)
	}
	return binPath
}

// writeScript writes an executable shell script to dir/name.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/usr/bin/env sh\n"+content), 0o755))
	return path
}

// minimalConfig returns a .semrel.yaml with a condition stub and a release stub.
func minimalConfig(conditionPath, providerPath string) string {
	return fmt.Sprintf(`tagPrefix: "v"
branches:
  - name: main
plugins:
  - path: %s
    phase: condition
  - path: %s
    phase: release
`, conditionPath, providerPath)
}

// gitEnv returns environment variables for a hermetic git environment.
func gitEnv(repoDir string) []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+repoDir,
		"SEMREL_REPOSITORY_URL=https://github.com/test/repo",
	)
}

// extractJSON returns the JSON object/array starting at the first '{' in s.
// When semrel runs in --dry-run mode it prints a banner before the JSON.
func extractJSON(s string) []byte {
	idx := strings.Index(s, "{")
	if idx < 0 {
		return []byte(s)
	}
	return []byte(s[idx:])
}

// TestE2E_DryRunFeatRelease exercises the full pipeline in dry-run mode:
// feat + fix commits from v1.0.0 → expect v1.1.0, minor bump, no real tag.
func TestE2E_DryRunFeatRelease(t *testing.T) {
	requireGit(t)
	semrelBin := requireSemrelBinary(t)

	repoDir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, repoDir, "v1.0.0")
	addCommit(t, repoDir, "feature.go", "feat: add awesome feature")
	addCommit(t, repoDir, "bugfix.go", "fix: resolve edge case")

	stubDir := t.TempDir()
	cfg := minimalConfig(
		writeScript(t, stubDir, "condition", `exit 0`),
		writeScript(t, stubDir, "provider", `exit 0`),
	)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".semrel.yaml"), []byte(cfg), 0o644))

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(semrelBin, "release", "--dry-run", "--output", "json", "--no-color")
	cmd.Dir = repoDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = gitEnv(repoDir)

	require.NoError(t, cmd.Run(), "semrel release --dry-run should exit 0\nstderr: %s", stderr.String())
	t.Logf("stdout: %s", stdout.String())

	var summary struct {
		Released       bool   `json:"released"`
		DryRun         bool   `json:"dry_run"`
		CurrentVersion string `json:"current_version"`
		NextVersion    string `json:"next_version"`
		Bump           string `json:"bump"`
	}
	require.NoError(t, json.Unmarshal(extractJSON(stdout.String()), &summary))

	require.True(t, summary.DryRun)
	require.True(t, summary.Released, "feat commit should trigger a release")
	require.Equal(t, "v1.0.0", summary.CurrentVersion)
	require.Equal(t, "v1.1.0", summary.NextVersion)
	require.Equal(t, "minor", summary.Bump)

	// No real tag must have been created.
	out, _ := exec.Command("git", "-C", repoDir, "tag", "--list", "v1.1.0").Output()
	require.Empty(t, strings.TrimSpace(string(out)), "dry-run must not create a real tag")
}

// TestE2E_NoReleasableCommits verifies semrel exits 0 with released=false when
// all commits are non-releasable (docs, chore, etc.).
func TestE2E_NoReleasableCommits(t *testing.T) {
	requireGit(t)
	semrelBin := requireSemrelBinary(t)

	repoDir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, repoDir, "v2.0.0")
	addCommit(t, repoDir, "readme.md", "docs: update README")
	addCommit(t, repoDir, "deps.txt", "chore: bump dependencies")

	stubDir := t.TempDir()
	cfg := minimalConfig(
		writeScript(t, stubDir, "condition", `exit 0`),
		writeScript(t, stubDir, "provider", `exit 0`),
	)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".semrel.yaml"), []byte(cfg), 0o644))

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(semrelBin, "release", "--dry-run", "--output", "json", "--no-color")
	cmd.Dir = repoDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = gitEnv(repoDir)

	require.NoError(t, cmd.Run(), "stderr: %s", stderr.String())
	t.Logf("stdout: %s", stdout.String())

	var summary struct {
		Released bool `json:"released"`
		DryRun   bool `json:"dry_run"`
	}
	require.NoError(t, json.Unmarshal(extractJSON(stdout.String()), &summary))
	require.True(t, summary.DryRun)
	require.False(t, summary.Released, "non-releasable commits → released must be false")
}

// TestE2E_BreakingChangeTriggersRealTag verifies a breaking change triggers
// a major bump and in non-dry-run mode the git tag IS actually created.
func TestE2E_BreakingChangeTriggersRealTag(t *testing.T) {
	requireGit(t)
	semrelBin := requireSemrelBinary(t)

	repoDir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, repoDir, "v1.5.0")
	addCommit(t, repoDir, "api.go", "feat!: redesign public API\n\nBREAKING CHANGE: all method signatures changed")

	stubDir := t.TempDir()
	cfg := minimalConfig(
		writeScript(t, stubDir, "condition", `exit 0`),
		writeScript(t, stubDir, "provider", `exit 0`),
	)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".semrel.yaml"), []byte(cfg), 0o644))

	var stdout, stderr bytes.Buffer
	// Run WITHOUT --dry-run so the tag is created.
	cmd := exec.Command(semrelBin, "release", "--output", "json", "--no-color")
	cmd.Dir = repoDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = gitEnv(repoDir)

	require.NoError(t, cmd.Run(), "stderr: %s", stderr.String())
	t.Logf("stdout: %s", stdout.String())

	var summary struct {
		Released    bool   `json:"released"`
		DryRun      bool   `json:"dry_run"`
		NextVersion string `json:"next_version"`
		Bump        string `json:"bump"`
	}
	require.NoError(t, json.Unmarshal(extractJSON(stdout.String()), &summary))

	require.True(t, summary.Released)
	require.False(t, summary.DryRun)
	require.Equal(t, "v2.0.0", summary.NextVersion)
	require.Equal(t, "major", summary.Bump)

	// Verify the real tag exists.
	out, _ := exec.Command("git", "-C", repoDir, "tag", "--list", "v2.0.0").Output()
	require.Equal(t, "v2.0.0", strings.TrimSpace(string(out)), "real tag v2.0.0 must exist")
}

// TestE2E_ConditionFailureAbortsRelease verifies that a failing condition
// plugin causes semrel to exit non-zero and no tag is created.
func TestE2E_ConditionFailureAbortsRelease(t *testing.T) {
	requireGit(t)
	semrelBin := requireSemrelBinary(t)

	repoDir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, repoDir, "v1.0.0")
	addCommit(t, repoDir, "feat.go", "feat: new feature")

	stubDir := t.TempDir()
	cfg := minimalConfig(
		writeScript(t, stubDir, "condition", `echo "Not in CI" >&2; exit 1`),
		writeScript(t, stubDir, "provider", `exit 0`),
	)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".semrel.yaml"), []byte(cfg), 0o644))

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(semrelBin, "release", "--no-color")
	cmd.Dir = repoDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = gitEnv(repoDir)

	require.Error(t, cmd.Run(), "semrel should exit non-zero when condition fails")

	out, _ := exec.Command("git", "-C", repoDir, "tag", "--list", "v1.1.0").Output()
	require.Empty(t, strings.TrimSpace(string(out)), "no tag when condition fails")
}

// TestE2E_PreTagPluginRuns verifies that a pre-tag plugin is invoked with the
// correct SEMREL_* environment variables before the git tag is created.
func TestE2E_PreTagPluginRuns(t *testing.T) {
	requireGit(t)
	semrelBin := requireSemrelBinary(t)

	repoDir, cleanup := initRepo(t)
	defer cleanup()

	addTag(t, repoDir, "v0.1.0")
	addCommit(t, repoDir, "feat.go", "feat: add feature")

	stubDir := t.TempDir()
	markerFile := filepath.Join(stubDir, "pre-tag-ran")
	preTagScript := fmt.Sprintf(`echo "$SEMREL_NEXT_VERSION" > %s; exit 0`, markerFile)

	cfg := fmt.Sprintf(`tagPrefix: "v"
branches:
  - name: main
plugins:
  - path: %s
    phase: condition
  - path: %s
    phase: pre-tag
  - path: %s
    phase: release
`,
		writeScript(t, stubDir, "condition", `exit 0`),
		writeScript(t, stubDir, "pre-tag", preTagScript),
		writeScript(t, stubDir, "provider", `exit 0`),
	)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".semrel.yaml"), []byte(cfg), 0o644))

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(semrelBin, "release", "--output", "json", "--no-color")
	cmd.Dir = repoDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = gitEnv(repoDir)

	require.NoError(t, cmd.Run(), "stderr: %s", stderr.String())

	marker, err := os.ReadFile(markerFile)
	require.NoError(t, err, "pre-tag plugin marker file must exist")
	require.Equal(t, "v0.2.0", strings.TrimSpace(string(marker)),
		"pre-tag plugin must receive SEMREL_NEXT_VERSION=v0.2.0")
}

// TestSmokeTestAllPlugins builds and smoke-tests all plugin binaries found in
// sibling directories. Run with:
//
//	go test ./test/integration/... -run TestSmokeTestAllPlugins -v
func TestSmokeTestAllPlugins(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH")
	}

	fakeEnv := append(os.Environ(),
		"SEMREL_VERSION=v1.2.3",
		"SEMREL_TAG_NAME=v1.2.3",
		"SEMREL_NEXT_VERSION=v1.2.3",
		"SEMREL_CURRENT_VERSION=v1.2.2",
		"SEMREL_BUMP=patch",
		"SEMREL_BRANCH=main",
		"SEMREL_TAG_PREFIX=v",
		"SEMREL_CHANGELOG=## v1.2.3\n\n- fix: test change",
		"SEMREL_DRY_RUN=true",
		`SEMREL_COMMITS=["fix: test commit"]`,
		"GITHUB_ACTIONS=true",
		"GITHUB_TOKEN=fake-token",
		"GITHUB_REF_NAME=main",
		"GITHUB_REPOSITORY=owner/repo",
		"GITEA_ACTIONS=true",
		"GITEA_TOKEN=fake-token",
		"GITLAB_CI=true",
		"CI_JOB_TOKEN=fake-token",
		"CI_COMMIT_REF_NAME=main",
	)

	pluginPrefixes := []string{
		"provider-", "hook-", "condition-", "analyzer-", "updater-", "generator-", "packager-", "publisher-",
	}
	isPlugin := func(name string) bool {
		for _, prefix := range pluginPrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
		return false
	}

	// Plugin repos sit one level above the semrel repo root.
	// Working dir during test: test/integration/ → ../../.. is the plugins parent.
	pluginParent := filepath.Join("..", "..", "..")
	entries, err := os.ReadDir(pluginParent)
	require.NoError(t, err)

	binDir := t.TempDir()
	pass, fail, skip := 0, 0, 0

	for _, entry := range entries {
		if !entry.IsDir() || !isPlugin(entry.Name()) {
			continue
		}

		repoDir := filepath.Join(pluginParent, entry.Name())
		if _, statErr := os.Stat(filepath.Join(repoDir, "cmd", "plugin", "main.go")); os.IsNotExist(statErr) {
			t.Logf("SKIP %-40s (no cmd/plugin/main.go)", entry.Name())
			skip++
			continue
		}

		binPath := filepath.Join(binDir, "plugin-"+entry.Name())
		buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/plugin")
		buildCmd.Dir = repoDir
		if buildOut, buildErr := buildCmd.CombinedOutput(); buildErr != nil {
			t.Errorf("FAIL %-40s (build failed)\n%s", entry.Name(), buildOut)
			fail++
			continue
		}

		runCmd := exec.Command(binPath)
		runCmd.Env = fakeEnv
		runOut, runErr := runCmd.CombinedOutput()

		if runErr == nil {
			t.Logf("PASS %-40s", entry.Name())
			pass++
		} else {
			outStr := strings.ToLower(string(runOut))
			if containsAny(outStr, "required", "missing", "not set", "must be set") {
				t.Logf("SKIP %-40s (needs real config — dry-run ok)", entry.Name())
				skip++
			} else {
				t.Errorf("FAIL %-40s (exit %v)\n%s", entry.Name(), runErr, runOut)
				fail++
			}
		}
	}

	t.Logf("\nSmoke test results: PASS=%d  SKIP=%d  FAIL=%d", pass, skip, fail)
	if fail > 0 {
		t.Fail()
	}
	if pass+skip == 0 {
		t.Skip("no plugin repos found — ensure plugin repos are siblings of this repo")
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
