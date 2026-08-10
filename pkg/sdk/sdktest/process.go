// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package sdktest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Standard semrel plugin environment variable names.
const (
	EnvCurrentVersion = "SEMREL_CURRENT_VERSION"
	EnvVersion        = "SEMREL_VERSION"
	EnvTagName        = "SEMREL_TAG_NAME"
	EnvNextVersion    = "SEMREL_NEXT_VERSION"
	EnvBump           = "SEMREL_BUMP"
	EnvTagPrefix      = "SEMREL_TAG_PREFIX"
	EnvChangelog      = "SEMREL_CHANGELOG"
	EnvBranch         = "SEMREL_BRANCH"
	EnvDryRun         = "SEMREL_DRY_RUN"
	EnvCommits        = "SEMREL_COMMITS"
	EnvRepositoryURL  = "SEMREL_REPOSITORY_URL"
)

const defaultProcessTimeout = 30 * time.Second

var pluginKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ReleaseEnvironment is the environment contract passed by semrel to a plugin.
type ReleaseEnvironment struct {
	CurrentVersion string
	Version        string
	TagName        string
	NextVersion    string
	Bump           string
	TagPrefix      string
	Changelog      string
	Branch         string
	DryRun         bool
	Commits        []string
	RepositoryURL  string
}

// Variables returns the SEMREL_* release environment. Empty core values are
// retained so tests can verify missing-configuration behavior. Commits and
// repository URL are omitted unless configured, matching the core runner.
func (environment ReleaseEnvironment) Variables() (map[string]string, error) {
	variables := map[string]string{
		EnvCurrentVersion: environment.CurrentVersion,
		EnvVersion:        environment.Version,
		EnvTagName:        environment.TagName,
		EnvNextVersion:    environment.NextVersion,
		EnvBump:           environment.Bump,
		EnvTagPrefix:      environment.TagPrefix,
		EnvChangelog:      environment.Changelog,
		EnvBranch:         environment.Branch,
		EnvDryRun:         fmt.Sprintf("%t", environment.DryRun),
	}
	if environment.Commits != nil {
		commits, err := json.Marshal(environment.Commits)
		if err != nil {
			return nil, fmt.Errorf("encode SEMREL_COMMITS: %w", err)
		}
		variables[EnvCommits] = string(commits)
	}
	if environment.RepositoryURL != "" {
		variables[EnvRepositoryURL] = environment.RepositoryURL
	}
	return variables, nil
}

// PluginVariables converts plugin configuration keys such as "webhook-url" or
// "SEMREL_PLUGIN_WEBHOOK_URL" to their canonical environment names.
func PluginVariables(config map[string]string) (map[string]string, error) {
	variables := make(map[string]string, len(config))
	for key, value := range config {
		if value == "" {
			continue
		}
		key = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(key)), "SEMREL_PLUGIN_")
		key = strings.NewReplacer("-", "_", ".", "_").Replace(key)
		if !pluginKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid plugin environment key %q", key)
		}
		variables["SEMREL_PLUGIN_"+key] = value
	}
	return variables, nil
}

// RunOptions configures a plugin subprocess.
type RunOptions struct {
	Args         []string
	Dir          string
	Stdin        []byte
	Environment  ReleaseEnvironment
	PluginConfig map[string]string
	Env          map[string]string
	Timeout      time.Duration
}

// ProcessResult captures every observable part of a plugin subprocess.
type ProcessResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
	Canceled bool
	Err      error
}

// Success reports whether the process started, completed before its deadline,
// and exited with status zero.
func (result ProcessResult) Success() bool {
	return result.Err == nil && !result.TimedOut && result.ExitCode == 0
}

// CheckNoSecretOutput verifies that stdout and stderr do not contain any of
// the supplied non-empty secrets. Error messages identify only the stream and
// secret index so the diagnostic cannot leak the secret itself.
func (result ProcessResult) CheckNoSecretOutput(secrets ...string) error {
	for index, secret := range secrets {
		if secret == "" {
			continue
		}
		if strings.Contains(result.Stdout, secret) {
			return fmt.Errorf("stdout contains secret %d", index+1)
		}
		if strings.Contains(result.Stderr, secret) {
			return fmt.Errorf("stderr contains secret %d", index+1)
		}
	}
	return nil
}

// RunPlugin executes a plugin with an isolated SEMREL_* environment. Existing
// SEMREL_* variables are removed before the configured contract is appended.
// The child is killed and waited for when the timeout or parent context ends.
func RunPlugin(ctx context.Context, path string, options RunOptions) ProcessResult {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultProcessTimeout
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	releaseVariables, err := options.Environment.Variables()
	if err != nil {
		return ProcessResult{ExitCode: -1, Err: err}
	}
	pluginVariables, err := PluginVariables(options.PluginConfig)
	if err != nil {
		return ProcessResult{ExitCode: -1, Err: err}
	}
	for key, value := range pluginVariables {
		releaseVariables[key] = value
	}
	for key, value := range options.Env {
		releaseVariables[key] = value
	}

	command := exec.CommandContext(runContext, path, options.Args...)
	command.Dir = options.Dir
	command.Env = appendWithoutSemrel(os.Environ(), releaseVariables)
	command.Stdin = bytes.NewReader(options.Stdin)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	result := ProcessResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: -1,
		Err:      runErr,
	}
	if runContext.Err() != nil {
		result.TimedOut = errors.Is(runContext.Err(), context.DeadlineExceeded)
		result.Canceled = errors.Is(runContext.Err(), context.Canceled)
		result.ExitCode = -1
		result.Err = runContext.Err()
		return result
	}
	if runErr == nil {
		result.ExitCode = 0
		return result
	}
	var exitError *exec.ExitError
	if errors.As(runErr, &exitError) {
		result.ExitCode = exitError.ExitCode()
	}
	return result
}

func appendWithoutSemrel(base []string, variables map[string]string) []string {
	environment := make([]string, 0, len(base))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "SEMREL_") {
			environment = append(environment, entry)
		}
	}
	for name, value := range variables {
		environment = append(environment, name+"="+value)
	}
	return environment
}
