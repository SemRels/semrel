//go:build integration

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package sdktest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SemRels/semrel/pkg/sdk"
	"github.com/SemRels/semrel/pkg/sdk/sdktest"
)

type sdkPilotPlugin struct{}

func (sdkPilotPlugin) Name() string    { return "sdk-pilot" }
func (sdkPilotPlugin) Version() string { return "1.0.0" }
func (sdkPilotPlugin) PreRelease(_ context.Context, _ sdk.Config) error {
	return nil
}
func (sdkPilotPlugin) PostRelease(_ context.Context, event sdk.ReleaseEvent) error {
	if event.Version == "fail" {
		return errors.New("pilot failure")
	}
	return nil
}

func TestPluginProcessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SDKTEST_PROCESS") != "1" {
		return
	}
	switch os.Getenv("SDKTEST_MODE") {
	case "environment":
		names := strings.Split(os.Getenv("SDKTEST_ENV_NAMES"), ",")
		values := make(map[string]string, len(names))
		for _, name := range names {
			values[name] = os.Getenv(name)
		}
		_ = json.NewEncoder(os.Stdout).Encode(values)
		_, _ = fmt.Fprintln(os.Stderr, "helper stderr")
		os.Exit(7)
	case "timeout":
		_, _ = fmt.Fprintln(os.Stderr, "helper started")
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "sdk":
		sdk.Run(sdkPilotPlugin{})
		os.Exit(0)
	default:
		os.Exit(9)
	}
}

func TestReleaseEnvironmentAndProcessOutput(t *testing.T) {
	t.Setenv("SEMREL_VERSION", "inherited-value-must-not-leak")
	names := []string{
		sdktest.EnvCurrentVersion,
		sdktest.EnvVersion,
		sdktest.EnvTagName,
		sdktest.EnvNextVersion,
		sdktest.EnvBump,
		sdktest.EnvTagPrefix,
		sdktest.EnvChangelog,
		sdktest.EnvBranch,
		sdktest.EnvDryRun,
		sdktest.EnvCommits,
		sdktest.EnvRepositoryURL,
		"SEMREL_PLUGIN_TOKEN",
	}
	result := sdktest.RunPlugin(context.Background(), os.Args[0], sdktest.RunOptions{
		Args: []string{"-test.run=^TestPluginProcessHelper$"},
		Environment: sdktest.ReleaseEnvironment{
			CurrentVersion: "v1.2.2",
			Version:        "v1.2.3",
			TagName:        "v1.2.3",
			NextVersion:    "v1.2.3",
			Bump:           "patch",
			TagPrefix:      "v",
			Changelog:      "fixed",
			Branch:         "main",
			DryRun:         true,
			Commits:        []string{"fix: bug"},
			RepositoryURL:  "https://example.test/org/repo",
		},
		PluginConfig: map[string]string{"token": "secret"},
		Env: map[string]string{
			"GO_WANT_SDKTEST_PROCESS": "1",
			"SDKTEST_MODE":            "environment",
			"SDKTEST_ENV_NAMES":       strings.Join(names, ","),
		},
	})

	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7 (err=%v)", result.ExitCode, result.Err)
	}
	if result.Success() {
		t.Fatal("non-zero process reported success")
	}
	if result.Stderr != "helper stderr\n" {
		t.Fatalf("stderr = %q", result.Stderr)
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(result.Stdout), &values); err != nil {
		t.Fatalf("decode stdout %q: %v", result.Stdout, err)
	}
	want := map[string]string{
		sdktest.EnvCurrentVersion: "v1.2.2",
		sdktest.EnvVersion:        "v1.2.3",
		sdktest.EnvTagName:        "v1.2.3",
		sdktest.EnvNextVersion:    "v1.2.3",
		sdktest.EnvBump:           "patch",
		sdktest.EnvTagPrefix:      "v",
		sdktest.EnvChangelog:      "fixed",
		sdktest.EnvBranch:         "main",
		sdktest.EnvDryRun:         "true",
		sdktest.EnvCommits:        `["fix: bug"]`,
		sdktest.EnvRepositoryURL:  "https://example.test/org/repo",
		"SEMREL_PLUGIN_TOKEN":     "secret",
	}
	for name, expected := range want {
		if values[name] != expected {
			t.Errorf("%s = %q, want %q", name, values[name], expected)
		}
	}
}

func TestRunPluginTimeoutWaitsForCleanup(t *testing.T) {
	started := time.Now()
	result := sdktest.RunPlugin(context.Background(), os.Args[0], sdktest.RunOptions{
		Args: []string{"-test.run=^TestPluginProcessHelper$"},
		Env: map[string]string{
			"GO_WANT_SDKTEST_PROCESS": "1",
			"SDKTEST_MODE":            "timeout",
		},
		Timeout: 100 * time.Millisecond,
	})
	if !result.TimedOut || !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("timeout result = %#v", result)
	}
	if result.ExitCode != -1 {
		t.Fatalf("timeout exit code = %d, want -1", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "helper started") {
		t.Fatalf("stderr = %q", result.Stderr)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("timed-out process cleanup took %s", elapsed)
	}
}

func TestRunPluginHonorsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := sdktest.RunPlugin(ctx, os.Args[0], sdktest.RunOptions{
		Args: []string{"-test.run=^TestPluginProcessHelper$"},
		Env: map[string]string{
			"GO_WANT_SDKTEST_PROCESS": "1",
			"SDKTEST_MODE":            "timeout",
		},
	})
	if !result.Canceled || result.TimedOut || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("cancellation result = %#v", result)
	}
	if result.ExitCode != -1 {
		t.Fatalf("cancellation exit code = %d, want -1", result.ExitCode)
	}
}

func TestRunPluginRejectsInvalidPluginEnvironmentKey(t *testing.T) {
	result := sdktest.RunPlugin(context.Background(), os.Args[0], sdktest.RunOptions{
		PluginConfig: map[string]string{"": "invalid"},
	})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "invalid plugin environment key") {
		t.Fatalf("error = %v", result.Err)
	}
}

func TestRunPluginReportsStartFailureWithoutExitStatus(t *testing.T) {
	result := sdktest.RunPlugin(context.Background(), "sdktest-binary-does-not-exist", sdktest.RunOptions{})
	if result.Err == nil {
		t.Fatal("missing binary unexpectedly started")
	}
	if result.ExitCode != -1 || result.Success() {
		t.Fatalf("start failure result = %#v", result)
	}
}

func TestProcessResultChecksSecretRedaction(t *testing.T) {
	if err := (sdktest.ProcessResult{Stdout: "published", Stderr: "safe diagnostic"}).
		CheckNoSecretOutput("top-secret"); err != nil {
		t.Fatalf("safe output rejected: %v", err)
	}
	err := (sdktest.ProcessResult{Stderr: "request failed: top-secret"}).
		CheckNoSecretOutput("", "top-secret")
	if err == nil || err.Error() != "stderr contains secret 2" {
		t.Fatalf("redaction error = %v", err)
	}
	if strings.Contains(err.Error(), "top-secret") {
		t.Fatal("redaction error leaked the secret")
	}
}

func TestSDKSubprocessPilot(t *testing.T) {
	request, err := json.Marshal(sdk.Request{
		Action: "post-release",
		Event:  sdk.ReleaseEvent{Version: "v1.2.3", TagName: "v1.2.3", DryRun: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := sdktest.RunPlugin(context.Background(), os.Args[0], sdktest.RunOptions{
		Args:  []string{"-test.run=^TestPluginProcessHelper$"},
		Stdin: request,
		Env: map[string]string{
			"GO_WANT_SDKTEST_PROCESS": "1",
			"SDKTEST_MODE":            "sdk",
		},
	})
	if !result.Success() {
		t.Fatalf("SDK pilot failed: exit=%d err=%v stderr=%s", result.ExitCode, result.Err, result.Stderr)
	}
	if result.Stderr != "" {
		t.Fatalf("SDK pilot stderr = %q", result.Stderr)
	}
	var response sdk.Response
	if err := json.Unmarshal([]byte(result.Stdout), &response); err != nil {
		t.Fatalf("decode SDK response %q: %v", result.Stdout, err)
	}
	if !response.Success || !strings.Contains(response.Message, "post-release completed successfully") {
		t.Fatalf("SDK response = %#v", response)
	}
}
