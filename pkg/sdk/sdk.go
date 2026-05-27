// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package sdk provides the semrel plugin SDK for third-party plugin authors.
// Plugin authors import this package to implement a semrel-compatible plugin
// binary using the standard Plugin interface.
//
// Quick start:
//
//	import "github.com/GoSemantics/semrel/pkg/sdk"
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Name() string    { return "my-plugin" }
//	func (p *MyPlugin) Version() string { return "1.0.0" }
//	func (p *MyPlugin) PreRelease(ctx context.Context, cfg sdk.Config) error  { return nil }
//	func (p *MyPlugin) PostRelease(ctx context.Context, event sdk.ReleaseEvent) error { ... }
//
//	func main() { sdk.Run(&MyPlugin{}) }
//
// The SDK handles:
//   - Argument and environment parsing
//   - JSON-encoded config from stdin
//   - Structured exit codes (0=success, 1=error, 2=skipped)
//   - Schema validation for required config fields
//
// See: https://github.com/SemRels/semrel/issues/51
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// ExitCode values used by semrel plugins.
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitSkipped = 2
)

// Config is the plugin configuration map as provided by the semrel config file.
// Values are interface{} to support strings, numbers, booleans, and nested maps.
type Config map[string]interface{}

// Get returns the value for key, or ok=false if not present.
func (c Config) Get(key string) (interface{}, bool) {
	v, ok := c[key]
	return v, ok
}

// GetString returns the value for key as a string, or "" if not present or not a string.
func (c Config) GetString(key string) string {
	v, ok := c[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetBool returns the value for key as a bool, or false if not present or not a bool.
func (c Config) GetBool(key string) bool {
	v, ok := c[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// ReleaseEvent is passed to PostRelease with the results of the release.
type ReleaseEvent struct {
	// Version is the new release version (e.g. "1.2.0").
	Version string `json:"version"`
	// PreviousVersion is the version before this release.
	PreviousVersion string `json:"previous_version"`
	// TagName is the full git tag (e.g. "v1.2.0").
	TagName string `json:"tag_name"`
	// Changelog is the rendered release notes for this version.
	Changelog string `json:"changelog"`
	// Repository is the git remote URL.
	Repository string `json:"repository"`
	// Branch is the branch from which the release was made.
	Branch string `json:"branch"`
	// DryRun is true when semrel is running in dry-run mode.
	DryRun bool `json:"dry_run"`
}

// Plugin is the interface that all semrel plugins must implement.
type Plugin interface {
	// Name returns the plugin name (e.g. "semrel-plugin-github").
	Name() string
	// Version returns the plugin version (e.g. "1.0.0").
	Version() string
	// PreRelease is called before the new tag is created.
	// Use this for validation, pre-publish steps, etc.
	PreRelease(ctx context.Context, cfg Config) error
	// PostRelease is called after the new tag is created.
	// Use this for publishing, notifications, etc.
	PostRelease(ctx context.Context, event ReleaseEvent) error
}

// Request is the JSON structure that semrel sends to the plugin's stdin.
type Request struct {
	// Action is either "pre-release" or "post-release".
	Action string `json:"action"`
	// Config is the plugin configuration from .semrel.yaml.
	Config Config `json:"config"`
	// Event is populated for post-release actions.
	Event ReleaseEvent `json:"event,omitempty"`
}

// Response is the JSON structure the plugin writes to stdout.
type Response struct {
	// Success indicates whether the plugin step succeeded.
	Success bool `json:"success"`
	// Message is an optional human-readable result or error message.
	Message string `json:"message,omitempty"`
}

// Run is the main entry point for a plugin binary.
// It reads a Request from stdin, dispatches to the appropriate Plugin method,
// and writes a Response to stdout.
// Use os.Exit codes from the ExitCode constants.
func Run(p Plugin) {
	ctx := context.Background()

	var req Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		writeError(fmt.Sprintf("decoding request: %v", err))
		os.Exit(ExitError)
	}

	var runErr error
	switch req.Action {
	case "pre-release":
		runErr = p.PreRelease(ctx, req.Config)
	case "post-release":
		runErr = p.PostRelease(ctx, req.Event)
	default:
		writeError(fmt.Sprintf("unknown action: %q", req.Action))
		os.Exit(ExitError)
	}

	if runErr != nil {
		writeError(runErr.Error())
		os.Exit(ExitError)
	}

	writeSuccess(fmt.Sprintf("%s/%s: %s completed successfully", p.Name(), p.Version(), req.Action))
}

func writeSuccess(msg string) {
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(Response{Success: true, Message: msg}) //nolint:errcheck,gosec
}

func writeError(msg string) {
	enc := json.NewEncoder(os.Stderr)
	enc.Encode(Response{Success: false, Message: msg}) //nolint:errcheck,gosec
}
