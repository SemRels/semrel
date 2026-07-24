// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	conditionPluginVersion = "0.4.0"
	conditionPluginCommit  = "2ca699a4eacd762a6cbea9562d4b2dbe7ad75315"
	providerPluginVersion  = "0.3.0"
	providerPluginCommit   = "722a1e3e4e22f3adcf69398d8b966a33037bfee9"
)

type builtPlugin struct {
	category string
	artifact string
	legacy   string
	version  string
	commit   string
	released string
	aliases  []string
	data     []byte
}

type registryFixture struct {
	server           *httptest.Server
	plugins          map[string]builtPlugin
	providerFailures atomic.Int32
	requests         map[string]*atomic.Int32
	badChecksum      bool
}

func TestCoreRegistryReleasePathWithActualPlugins(t *testing.T) {
	requireGit(t)
	semrelBin := requireCoreSemrelBinary(t)
	plugins := buildActualPlugins(t)
	registry := newRegistryFixture(t, plugins, 2, false)
	cacheDir := t.TempDir()

	repoDir, cleanup := initRepo(t)
	defer cleanup()
	bareRemote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, "", "init", "--bare", bareRemote)
	runGit(t, repoDir, "remote", "add", "origin", bareRemote)
	addTag(t, repoDir, "v1.0.0")
	addCommit(t, repoDir, "feature.go", "feat: exercise canonical registry release")
	writeReleaseConfig(t, repoDir, "pass")

	env := append(hermeticPluginEnv(t, gitEnv(repoDir), t.TempDir(), "git"),
		"SEMREL_REGISTRY_URL="+registry.server.URL,
		"SEMREL_CACHE_DIR="+cacheDir,
		"SEMREL_ANALYTICS_FILE="+filepath.Join(t.TempDir(), "analytics.jsonl"),
		"E2E_GATE=pass",
	)
	for _, ref := range []string{
		"@semrel/condition-generic@" + conditionPluginVersion,
		"@semrel/provider-git@" + providerPluginVersion,
	} {
		stdout, stderr, err := runSemrel(semrelBin, repoDir, env, "plugin", "install", ref)
		require.NoError(t, err, "install %s\nstdout: %s\nstderr: %s", ref, stdout, stderr)
	}

	lockData, err := os.ReadFile(filepath.Join(repoDir, ".semrel.lock"))
	require.NoError(t, err)
	var lockFile struct {
		Plugins []struct {
			BinaryName string `json:"binaryName"`
			Ref        string `json:"ref"`
			Version    string `json:"version"`
		} `json:"plugins"`
	}
	require.NoError(t, json.Unmarshal(lockData, &lockFile))
	require.Len(t, lockFile.Plugins, 2)
	expectedLocks := map[string]struct {
		version string
		binary  string
	}{
		"@semrel/condition-generic": {conditionPluginVersion, "semrel-plugin-generic"},
		"@semrel/provider-git":      {providerPluginVersion, "semrel-plugin-git"},
	}
	for _, entry := range lockFile.Plugins {
		expected, ok := expectedLocks[entry.Ref]
		require.True(t, ok, "unexpected lock ref %s", entry.Ref)
		assert.Equal(t, expected.version, entry.Version)
		assert.Equal(t, expected.binary, entry.BinaryName)
	}
	assert.EqualValues(t, 3, registry.requests["provider-git"].Load(),
		"provider download should retry two transient 500 responses")

	platform := runtime.GOOS + "_" + runtime.GOARCH
	for _, plugin := range plugins {
		cachePath := filepath.Join(cacheDir, platform, plugin.legacy, plugin.version)
		entries, readErr := os.ReadDir(cachePath)
		require.NoError(t, readErr, "canonical cache path %s", cachePath)
		require.Len(t, entries, 1)
	}

	require.NoError(t, os.RemoveAll(filepath.Join(repoDir, ".semrel", "plugins")))
	offlineEnv := replaceEnv(env, "SEMREL_REGISTRY_URL", "http://127.0.0.1:1")
	stdout, stderr, err := runSemrel(semrelBin, repoDir, offlineEnv, "plugin", "restore")
	require.NoError(t, err, "offline cache restore\nstdout: %s\nstderr: %s", stdout, stderr)
	for _, plugin := range plugins {
		assert.Contains(t, stdout, "restored @semrel/"+plugin.artifact+"@"+plugin.version)
		binaryPath := filepath.Join(repoDir, ".semrel", "plugins",
			executableName("semrel-plugin-"+plugin.legacy))
		restored, readErr := os.ReadFile(binaryPath)
		require.NoError(t, readErr)
		assert.Equal(t, plugin.data, restored, "restored project-local binary %s", binaryPath)
	}

	stateBefore := snapshotReleaseState(t, repoDir, bareRemote)
	stdout, stderr, err = runSemrel(semrelBin, repoDir, offlineEnv,
		"release", "--dry-run", "--output", "json", "--no-color")
	require.NoError(t, err, "dry run\nstdout: %s\nstderr: %s", stdout, stderr)
	var drySummary struct {
		Released    bool   `json:"released"`
		DryRun      bool   `json:"dry_run"`
		NextVersion string `json:"next_version"`
	}
	require.NoError(t, json.Unmarshal(extractJSON(stdout), &drySummary))
	assert.True(t, drySummary.Released)
	assert.True(t, drySummary.DryRun)
	assert.Equal(t, "v1.1.0", drySummary.NextVersion)
	assertReleaseState(t, stateBefore, snapshotReleaseState(t, repoDir, bareRemote))
	_, err = os.Stat(filepath.Join(repoDir, "CHANGELOG.md"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Empty(t, gitOutput(t, repoDir, "tag", "--list", "v1.1.0"))
	assertRemoteRefMissing(t, bareRemote, "refs/tags/v1.1.0")

	stdout, stderr, err = runSemrel(semrelBin, repoDir, offlineEnv,
		"release", "--output", "json", "--no-color")
	require.NoError(t, err, "release\nstdout: %s\nstderr: %s", stdout, stderr)
	var releasedSummary struct {
		Released    bool   `json:"released"`
		NextVersion string `json:"next_version"`
	}
	require.NoError(t, json.Unmarshal(extractJSON(stdout), &releasedSummary))
	assert.True(t, releasedSummary.Released)
	assert.Equal(t, "v1.1.0", releasedSummary.NextVersion)
	assert.Equal(t, "v1.1.0", gitOutput(t, repoDir, "tag", "--list", "v1.1.0"))
	assertRemoteRefExists(t, bareRemote, "refs/tags/v1.1.0")
	assertRemoteRefExists(t, bareRemote, "refs/heads/main")

	stateBefore = snapshotReleaseState(t, repoDir, bareRemote)
	stdout, stderr, err = runSemrel(semrelBin, repoDir, offlineEnv,
		"release", "--output", "json", "--no-color")
	require.NoError(t, err, "idempotent rerun\nstdout: %s\nstderr: %s", stdout, stderr)
	var rerunSummary struct {
		Released bool `json:"released"`
	}
	require.NoError(t, json.Unmarshal(extractJSON(stdout), &rerunSummary))
	assert.False(t, rerunSummary.Released)
	assertReleaseState(t, stateBefore, snapshotReleaseState(t, repoDir, bareRemote))

	addCommit(t, repoDir, "next.go", "feat: condition must fail before mutation")
	writeReleaseConfig(t, repoDir, "blocked")
	stateBefore = snapshotReleaseState(t, repoDir, bareRemote)
	stdout, stderr, err = runSemrel(semrelBin, repoDir, offlineEnv,
		"release", "--output", "json", "--no-color")
	require.Error(t, err, "actual condition plugin must fail")
	assert.Contains(t, stderr+stdout, "condition check failed")
	assert.Empty(t, gitOutput(t, repoDir, "tag", "--list", "v1.2.0"))
	assertReleaseState(t, stateBefore, snapshotReleaseState(t, repoDir, bareRemote))

	t.Run("registry unavailable fails locked release before tag", func(t *testing.T) {
		freshRepo, freshCleanup := initRepo(t)
		defer freshCleanup()
		freshRemote := filepath.Join(t.TempDir(), "remote.git")
		runGit(t, "", "init", "--bare", freshRemote)
		runGit(t, freshRepo, "remote", "add", "origin", freshRemote)
		addTag(t, freshRepo, "v1.0.0")
		addCommit(t, freshRepo, "feature.go", "feat: unavailable registry")
		writeReleaseConfig(t, freshRepo, "pass")
		require.NoError(t, os.WriteFile(filepath.Join(freshRepo, ".semrel.lock"), lockData, 0o644))

		freshEnv := hermeticPluginEnv(t, gitEnv(freshRepo), t.TempDir(), "git")
		freshEnv = append(freshEnv,
			"SEMREL_REGISTRY_URL=http://127.0.0.1:1",
			"SEMREL_CACHE_DIR="+t.TempDir(),
			"SEMREL_ANALYTICS_FILE="+filepath.Join(t.TempDir(), "analytics.jsonl"),
			"E2E_GATE=pass",
		)
		before := snapshotReleaseState(t, freshRepo, freshRemote)
		stdout, stderr, err := runSemrel(semrelBin, freshRepo, freshEnv,
			"release", "--output", "json", "--no-color")
		require.Error(t, err)
		assert.Contains(t, stdout+stderr, "auto-restoring plugins")
		assert.Empty(t, gitOutput(t, freshRepo, "tag", "--list", "v1.1.0"))
		assertReleaseState(t, before, snapshotReleaseState(t, freshRepo, freshRemote))
	})

	t.Run("checksum failure leaves no install or lock", func(t *testing.T) {
		badRegistry := newRegistryFixture(t, plugins, 0, true)
		freshRepo := t.TempDir()
		freshEnv := hermeticPluginEnv(t, os.Environ(), t.TempDir())
		freshEnv = append(freshEnv,
			"SEMREL_REGISTRY_URL="+badRegistry.server.URL,
			"SEMREL_CACHE_DIR="+t.TempDir(),
		)
		stdout, stderr, err := runSemrel(semrelBin, freshRepo, freshEnv,
			"plugin", "install", "@semrel/condition-generic@"+conditionPluginVersion)
		require.Error(t, err)
		assert.Contains(t, stdout+stderr, "checksum mismatch")
		_, statErr := os.Stat(filepath.Join(freshRepo, ".semrel.lock"))
		assert.ErrorIs(t, statErr, os.ErrNotExist)
		_, statErr = os.Stat(filepath.Join(freshRepo, ".semrel", "plugins",
			executableName("semrel-plugin-generic")))
		assert.ErrorIs(t, statErr, os.ErrNotExist)
		assert.EqualValues(t, 1, badRegistry.requests["condition-generic"].Load(),
			"checksum errors are intentionally not retried")
	})
}

func buildActualPlugins(t *testing.T) map[string]builtPlugin {
	t.Helper()
	base := os.Getenv("SEMREL_PLUGIN_REPOS_DIR")
	if base == "" {
		var err error
		base, err = filepath.Abs(filepath.Join("..", "..", ".."))
		require.NoError(t, err)
	}

	definitions := []builtPlugin{
		{
			category: "condition",
			artifact: "condition-generic",
			legacy:   "generic",
			version:  conditionPluginVersion,
			commit:   conditionPluginCommit,
			released: "2026-06-24T07:01:55Z",
			aliases:  []string{"generic", "@semrel/generic", "condition-generic"},
		},
		{
			category: "provider",
			artifact: "provider-git",
			legacy:   "git",
			version:  providerPluginVersion,
			commit:   providerPluginCommit,
			released: "2026-06-24T07:09:06Z",
			aliases:  []string{"git", "@semrel/git", "provider-git"},
		},
	}
	result := make(map[string]builtPlugin, len(definitions))
	for _, definition := range definitions {
		repoDir := filepath.Join(base, definition.artifact)
		if _, err := os.Stat(filepath.Join(repoDir, "go.mod")); err != nil {
			if os.Getenv("SEMREL_PLUGIN_REPOS_DIR") != "" {
				t.Fatalf("required actual plugin repository %s: %v", repoDir, err)
			}
			t.Skipf("actual plugin repository not available at %s", repoDir)
		}
		head := gitOutput(t, repoDir, "rev-parse", "HEAD")
		require.Equal(t, definition.commit, head,
			"%s must be checked out at the commit mapped to v%s", definition.artifact, definition.version)
		output := filepath.Join(t.TempDir(), executableName(definition.artifact))
		cmd := exec.Command("go", "build", "-o", output, "./cmd/plugin")
		cmd.Dir = repoDir
		if buildOutput, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build actual plugin %s: %v\n%s", definition.artifact, err, buildOutput)
		}
		data, err := os.ReadFile(output)
		require.NoError(t, err)
		definition.data = data
		result[definition.artifact] = definition
	}
	return result
}

func requireCoreSemrelBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), executableName("semrel"))
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/semrel")
	cmd.Dir = filepath.Join("..", "..")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build semrel: %v\n%s", err, output)
	}
	return binPath
}

func newRegistryFixture(t *testing.T, plugins map[string]builtPlugin, providerFailures int32, badChecksum bool) *registryFixture {
	t.Helper()
	fixture := &registryFixture{
		plugins:     plugins,
		requests:    make(map[string]*atomic.Int32, len(plugins)),
		badChecksum: badChecksum,
	}
	fixture.providerFailures.Store(providerFailures)
	for name := range plugins {
		fixture.requests[name] = &atomic.Int32{}
	}

	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/plugins.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture.metadata())
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/artifacts/")
		name = strings.TrimSuffix(name, ".exe")
		plugin, ok := fixture.plugins[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fixture.requests[name].Add(1)
		if name == "provider-git" && fixture.providerFailures.Add(-1) >= 0 {
			http.Error(w, "transient registry failure", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(plugin.data)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *registryFixture) metadata() []byte {
	entries := make([]map[string]any, 0, len(f.plugins))
	for _, plugin := range f.plugins {
		checksum := sha256Hex(plugin.data)
		if f.badChecksum {
			checksum = strings.Repeat("0", 64)
		}
		entries = append(entries, map[string]any{
			"namespace":   "@semrel",
			"name":        plugin.artifact,
			"aliases":     plugin.aliases,
			"description": "E2E binary built from " + plugin.commit,
			"author":      "semrel Authors",
			"license":     "Apache-2.0",
			"category":    plugin.category,
			"repository":  "https://github.com/SemRels/" + plugin.artifact,
			"versions": []map[string]any{{
				"version":     plugin.version,
				"releaseDate": plugin.released,
				"downloadUrl": f.server.URL + "/artifacts/" + executableName(plugin.artifact),
				"checksums":   schemaV2Checksums(checksum),
				"prerelease":  false,
			}},
		})
	}
	data, err := json.Marshal(map[string]any{"schemaVersion": 2, "plugins": entries})
	if err != nil {
		panic(err)
	}
	return data
}

func schemaV2Checksums(checksum string) map[string]string {
	return map[string]string{
		"linux_amd64":   checksum,
		"linux_arm64":   checksum,
		"darwin_amd64":  checksum,
		"darwin_arm64":  checksum,
		"windows_amd64": checksum,
		"windows_arm64": checksum,
	}
}

func writeReleaseConfig(t *testing.T, repoDir, expectedGate string) {
	t.Helper()
	config := fmt.Sprintf(`tagPrefix: v
branches:
  - name: main
tag_exists_strategy: skip
plugins:
  - uses: "@semrel/condition-generic@%s"
    phase: condition
    args:
      env_var: E2E_GATE
      env_value: %s
  - uses: "@semrel/provider-git@%s"
    phase: release
`, conditionPluginVersion, expectedGate, providerPluginVersion)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".semrel.yaml"), []byte(config), 0o644))
}

func runSemrel(bin, dir string, env []string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v\n%s", args, out)
	return strings.TrimSpace(string(out))
}

func assertRemoteRefExists(t *testing.T, bareRemote, ref string) {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", bareRemote, "show-ref", "--verify", ref)
	require.NoError(t, cmd.Run(), ref)
}

func assertRemoteRefMissing(t *testing.T, bareRemote, ref string) {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", bareRemote, "show-ref", "--verify", ref)
	require.Error(t, cmd.Run(), ref)
}

type releaseState struct {
	head       string
	worktree   string
	remoteRefs string
}

func snapshotReleaseState(t *testing.T, repoDir, bareRemote string) releaseState {
	t.Helper()
	return releaseState{
		head:       gitOutput(t, repoDir, "rev-parse", "HEAD"),
		worktree:   worktreeSnapshot(t, repoDir),
		remoteRefs: gitOutput(t, "", "--git-dir", bareRemote, "for-each-ref", "--format=%(refname) %(objectname)"),
	}
}

func assertReleaseState(t *testing.T, expected, actual releaseState) {
	t.Helper()
	assert.Equal(t, expected.head, actual.head, "local HEAD changed")
	assert.Equal(t, expected.worktree, actual.worktree, "worktree content changed")
	assert.Equal(t, expected.remoteRefs, actual.remoteRefs, "bare-remote refs or hashes changed")
}

func worktreeSnapshot(t *testing.T, repoDir string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(repoDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(repoDir, path)
		if err != nil {
			return err
		}
		if relative == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s %04o %s",
			filepath.ToSlash(relative), info.Mode().Perm(), sha256Hex(data)))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(entries)
	return sha256Hex([]byte(strings.Join(entries, "\n")))
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(strings.ToUpper(entry), strings.ToUpper(prefix)) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func hermeticPluginEnv(t *testing.T, env []string, home string, requiredTools ...string) []string {
	t.Helper()
	pathDirs := make([]string, 0, len(requiredTools))
	seen := make(map[string]struct{}, len(requiredTools))
	for _, tool := range requiredTools {
		toolPath, err := exec.LookPath(tool)
		require.NoError(t, err, "required system tool %q", tool)
		dir := filepath.Dir(toolPath)
		key := strings.ToLower(dir)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			pathDirs = append(pathDirs, dir)
		}
	}
	env = replaceEnv(env, "HOME", home)
	env = replaceEnv(env, "USERPROFILE", home)
	return replaceEnv(env, "PATH", strings.Join(pathDirs, string(os.PathListSeparator)))
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
