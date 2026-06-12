# Plugin Development Guide

This guide explains how to develop custom plugins for semrel. Plugins extend semrel's functionality without modifying the core.

## Plugin Architecture Overview

semrel supports three types of plugins:

1. **Providers** — Publish releases to forges (GitHub, GitLab, etc.)
2. **Updaters** — Update version files and manifests
3. **Hooks** — Notify teams and log releases

All plugins are **standalone Go binaries** that communicate with semrel via a standard protocol.

## Getting Started

### Step 1: Use the Plugin Template

```bash
git clone https://github.com/SemRels/plugin-template.git semrel-plugin-myname
cd semrel-plugin-myname
```

### Step 2: Understand the Plugin Structure

```
semrel-plugin-myname/
├── cmd/
│   └── plugin/
│       └── main.go           # Entry point
├── internal/
│   └── plugin/
│       └── plugin.go         # Plugin implementation
├── schema/
│   └── config-schema.json    # Configuration schema (JSON Schema)
├── go.mod
├── go.sum
├── README.md
├── LICENSE
└── Dockerfile                # Multi-stage build for distribution
```

## Implementing a Plugin

### Example: A Simple Notification Hook Plugin

```go
// internal/plugin/plugin.go
package plugin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/SemRels/semrel/pkg/sdk"
)

type MyHookPlugin struct {
	logger *slog.Logger
}

// Execute is called during the release process.
// It receives the ReleaseEvent with version, changelog, and commit info.
func (p *MyHookPlugin) Execute(ctx context.Context, event sdk.ReleaseEvent) (*sdk.Result, error) {
	// Validate required configuration
	webhookURL, ok := event.Config["webhook_url"]
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	// Validate required event fields
	if event.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	p.logger.Info("Processing release",
		slog.String("version", event.Version),
		slog.String("branch", event.Repository.Branch),
	)

	// Perform the plugin action
	// Example: Send HTTP request to webhook
	message := fmt.Sprintf(
		"Released %s on %s\n\nChangelog:\n%s",
		event.Version,
		event.Repository.Branch,
		event.Changelog,
	)

	err := p.sendWebhook(ctx, webhookURL.(string), message)
	if err != nil {
		return nil, fmt.Errorf("webhook delivery failed: %w", err)
	}

	// Return results
	// Outputs are available to other plugins and the CLI
	return &sdk.Result{
		Outputs: map[string]string{
			"webhook_delivery": "success",
		},
	}, nil
}

func (p *MyHookPlugin) sendWebhook(ctx context.Context, url, message string) error {
	// Implementation: call webhook
	// ...
	return nil
}
```

### Entry Point

```go
// cmd/plugin/main.go
package main

import (
	"log/slog"
	"os"

	"github.com/SemRels/semrel/pkg/sdk"
	"semrel-plugin-myname/internal/plugin"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := &plugin.MyHookPlugin{
		logger: logger,
	}

	// sdk.Serve handles all communication with semrel
	if err := sdk.Serve(p); err != nil {
		logger.Error("Plugin failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
```

## Configuration Schema

Define what configuration your plugin accepts using JSON Schema:

```json
// schema/config-schema.json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "My Plugin Configuration",
  "type": "object",
  "properties": {
    "webhook_url": {
      "type": "string",
      "description": "Webhook URL to send notifications to"
    },
    "channel": {
      "type": "string",
      "description": "Channel or destination",
      "default": "#releases"
    },
    "include_changelog": {
      "type": "boolean",
      "description": "Include full changelog in notification",
      "default": true
    }
  },
  "required": ["webhook_url"],
  "additionalProperties": false
}
```

Users configure the plugin in `.semrel.yaml`:

```yaml
plugins:
  - uses: myname
    with:
      webhook_url: "https://example.com/webhook"
      channel: "#deployments"
      include_changelog: true
```

## Plugin Types

### Provider Plugin

Publishes releases to a forge (GitHub, GitLab, etc.).

**Responsibilities:**
- Create release/tag on the forge
- Upload artifacts if needed
- Return release URL and ID

**Example:**

```go
type GitHubProvider struct{}

func (p *GitHubProvider) Execute(ctx context.Context, event sdk.ReleaseEvent) (*sdk.Result, error) {
	// Authenticate with GitHub API
	token := event.Config["token"]

	// Create GitHub release via API
	releaseURL := "https://github.com/owner/repo/releases/tag/" + event.Version

	return &sdk.Result{
		Outputs: map[string]string{
			"release_url": releaseURL,
			"release_id": "12345",
		},
	}, nil
}
```

### Updater Plugin

Updates version files and dependency manifests.

**Responsibilities:**
- Locate version file(s)
- Update version information
- Optionally commit changes

**Example:**

```go
type NPMUpdater struct{}

func (p *NPMUpdater) Execute(ctx context.Context, event sdk.ReleaseEvent) (*sdk.Result, error) {
	// Read package.json
	// Update "version" field
	// Write package.json

	return &sdk.Result{
		Outputs: map[string]string{
			"files_updated": "package.json,package-lock.json",
		},
	}, nil
}
```

### Hook Plugin

Notifies teams and logs releases.

**Responsibilities:**
- Format notification message
- Send to external service (Slack, Teams, Email, etc.)
- Log delivery status

**Example:**

```go
type SlackHook struct{}

func (p *SlackHook) Execute(ctx context.Context, event sdk.ReleaseEvent) (*sdk.Result, error) {
	// Format Slack message
	message := &SlackMessage{
		Text: fmt.Sprintf("Released %s", event.Version),
		Blocks: []Block{
			// Rich formatting
		},
	}

	// Send to Slack API
	err := p.sendSlack(ctx, event.Config["webhook"].(string), message)
	if err != nil {
		// Don't fail release if webhook delivery fails
		// Just log it
		return nil, nil
	}

	return &sdk.Result{
		Outputs: map[string]string{
			"notification": "sent",
		},
	}, nil
}
```

## Testing Your Plugin

### Unit Tests

```go
func TestExecute(t *testing.T) {
	p := &MyHookPlugin{}
	event := sdk.ReleaseEvent{
		Version: "v1.2.0",
		Config: map[string]interface{}{
			"webhook_url": "https://example.com/webhook",
		},
	}

	result, err := p.Execute(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["webhook_delivery"] != "success" {
		t.Error("webhook delivery failed")
	}
}
```

### Integration Testing

Test your plugin with semrel:

```bash
# Build plugin
go build -o bin/semrel-plugin-myname ./cmd/plugin

# Create a test directory with .semrel.yaml
mkdir test-release
cd test-release

# Configure semrel to use your plugin
cat > .semrel.yaml <<EOF
schemaVersion: 1
branches:
  - name: main
plugins:
  - uses: myname
    with:
      webhook_url: "http://localhost:8080/webhook"
EOF

# Create git repo with commits
git init
git config user.email "test@example.com"
git config user.name "Test User"
git add .semrel.yaml
git commit -m "chore: initial commit"
git commit --allow-empty -m "feat: test feature"

# Run semrel (with dry-run to avoid side effects)
semrel release --dry-run
```

## Building and Distribution

### Building the Plugin Binary

```bash
# Build for current OS
make build

# Build for multiple platforms
make build-all

# This produces: bin/semrel-plugin-myname-{os}-{arch}
```

### Cross-Compilation

```bash
# Build for Linux x86_64
GOOS=linux GOARCH=amd64 go build -o bin/semrel-plugin-myname-linux-amd64 ./cmd/plugin

# Build for macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o bin/semrel-plugin-myname-darwin-arm64 ./cmd/plugin

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o bin/semrel-plugin-myname-windows-amd64.exe ./cmd/plugin
```

### Docker Distribution

semrel plugins should be distributed as Docker images (especially for CI/CD):

```dockerfile
# Dockerfile
FROM golang:1.20-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -o /plugin ./cmd/plugin

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /plugin /plugin
ENTRYPOINT ["/plugin"]
```

Build and push:

```bash
docker build -t ghcr.io/myorg/semrel-plugin-myname:v1.0.0 .
docker push ghcr.io/myorg/semrel-plugin-myname:v1.0.0
```

## Publishing to semrel Registry

Register your plugin in the [semrel-registry](https://github.com/SemRels/semrel-registry):

```yaml
# registry/plugins/myname.yaml
name: myname
description: "A short description of what your plugin does"
category: hook                          # provider, updater, or hook
author: "Your Organization"
license: "Apache-2.0"
repository: "https://github.com/org/semrel-plugin-myname"
documentation: "https://github.com/org/semrel-plugin-myname/blob/main/README.md"
versions:
  - version: "1.0.0"
    released: "2026-06-12"
    platforms:
      - os: linux
        arch: amd64
        download: "https://github.com/org/semrel-plugin-myname/releases/download/v1.0.0/semrel-plugin-myname-linux-amd64"
        checksum: "sha256:abc123..."
      - os: darwin
        arch: amd64
        download: "https://github.com/org/semrel-plugin-myname/releases/download/v1.0.0/semrel-plugin-myname-darwin-amd64"
        checksum: "sha256:def456..."
      # ... more platforms
```

Then users can install your plugin:

```bash
semrel plugin install myname
```

## Best Practices

### Error Handling

- Return errors for actual failures (can't authenticate, network error)
- Don't fail the release for non-critical issues (webhook delivery, logging)
- Provide helpful error messages

```go
// BAD: Fails release if webhook fails
return nil, fmt.Errorf("webhook failed: %w", err)

// GOOD: Logs failure but doesn't fail release
p.logger.Warn("webhook delivery failed", slog.String("error", err.Error()))
return &sdk.Result{}, nil
```

### Configuration Validation

Validate early and provide clear error messages:

```go
func (p *MyPlugin) Execute(ctx context.Context, event sdk.ReleaseEvent) (*sdk.Result, error) {
	// Validate all required fields
	token, ok := event.Config["token"]
	if !ok || token == "" {
		return nil, fmt.Errorf("configuration error: 'token' is required")
	}

	url, ok := event.Config["webhook_url"]
	if !ok || url == "" {
		return nil, fmt.Errorf("configuration error: 'webhook_url' is required")
	}

	// Continue with execution
	// ...
}
```

### Logging

Use structured logging for debugging:

```go
p.logger.Info("processing release",
	slog.String("version", event.Version),
	slog.String("branch", event.Repository.Branch),
	slog.Bool("is_prerelease", event.IsPrerelease),
)

p.logger.Debug("webhook details",
	slog.String("url", webhookURL),
	slog.String("message", message),
)
```

### Dependencies

Keep dependencies minimal to reduce binary size and complexity:

```bash
# Check dependencies
go mod graph

# Remove unused
go mod tidy

# Vendor if needed
go mod vendor
```

### Documentation

Provide clear README.md with:
- What the plugin does
- Configuration options
- Usage examples
- Troubleshooting tips

## Plugin Lifecycle

Plugins are **stateless**. Each invocation:
1. Process starts
2. Plugin receives event
3. Plugin returns result
4. Process exits

Don't store state between releases. Configuration is passed fresh each time.

## Advanced Topics

### Parallel Execution

Multiple plugin instances can run in parallel (with same type, different configs):

```yaml
plugins:
  - uses: slack
    name: slack_dev
    with:
      webhook: "https://..."
  - uses: slack
    name: slack_prod
    with:
      webhook: "https://..."
```

Both are executed (sequentially or in parallel depending on semrel config).

### Plugin Outputs

Plugins return outputs that can be used by subsequent plugins or the CLI:

```go
return &sdk.Result{
	Outputs: map[string]string{
		"release_url": "https://github.com/org/repo/releases/tag/v1.0.0",
		"release_id": "12345",
	},
}, nil
```

Outputs are available to downstream plugins and in CLI output (JSON format).

### Skip Plugin

A plugin can request to be skipped:

```go
return &sdk.Result{
	Skipped: true,
	SkipReason: "no changes since last release",
}, nil
```

## Troubleshooting

### Plugin Not Found

```
error loading plugin: not found in ~/.semrel/plugins or $PATH
```

**Solution:** Install plugin or ensure binary is in ~/.semrel/plugins/semrel-plugin-{name}

### Configuration Error

```
error: configuration error: 'token' is required
```

**Solution:** Add missing configuration to .semrel.yaml

### Plugin Crash

```
plugin exited with code 1
```

**Solution:** Check plugin logs (stderr output). Add debugging to your plugin.

## Examples

Complete examples are available in the [semrel-plugins](https://github.com/SemRels/semrel-plugins) repository:
- [hook-slack](https://github.com/SemRels/hook-slack)
- [hook-teams](https://github.com/SemRels/hook-teams)
- [provider-github](https://github.com/SemRels/provider-github)
- [updater-npm](https://github.com/SemRels/updater-npm)
- [updater-python](https://github.com/SemRels/updater-python)

## Support

- **Questions?** GitHub Discussions in plugin repository
- **Issues?** GitHub Issues with logs and config
- **SDK Changes?** Watch main semrel repository
- **Maintainers?** Reach out to @SemRels/maintainers

## License

Plugins should be licensed under a permissive license (MIT, Apache 2.0, etc.) to encourage use.

SPDX: Apache-2.0 (recommended)

