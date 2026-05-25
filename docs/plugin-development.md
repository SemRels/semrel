# Plugin Development Guide

This guide explains how to write a semrel plugin. It covers the plugin transport,
stdout/stderr contract, lifecycle hooks, and the gRPC API.

## Architecture Overview

semrel uses [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) as the
plugin transport layer. Each plugin runs as a **separate process**, communicating
with the semrel host via **gRPC over a Unix socket** (or named pipe on Windows).

```
semrel (host)
    │
    ├─ spawns ──▶ plugin-process (plugin binary)
    │                │
    │◄──── gRPC ─────┤  (VerifyConditions / AnalyzeCommits / GenerateNotes / ...)
    │                │
    └─────────────────
```

## ⚠ Critical: stdout/stderr Contract

**Plugins MUST NEVER write to stdout.**

The host process reads the plugin's `stdout` at startup to receive the gRPC
socket address (the go-plugin handshake). Any write to `stdout` before or after
this handshake corrupts the protocol and silently breaks the connection.

### Rules

| Stream   | Purpose                                        | Allowed?      |
|----------|------------------------------------------------|---------------|
| `stdout` | go-plugin handshake (socket address)           | ❌ Reserved   |
| `stderr` | All plugin logs, warnings, errors              | ✅ Use this   |

### Logging with `hclog`

Use [hashicorp/go-hclog](https://github.com/hashicorp/go-hclog) and set `Output: os.Stderr`:

```go
import (
    "os"
    hclog "github.com/hashicorp/go-hclog"
)

logger := hclog.New(&hclog.LoggerOptions{
    Name:   "my-plugin",
    Output: os.Stderr,  // MUST be stderr
    Level:  hclog.Info,
})
logger.Info("plugin started")
```

Never use:
- `fmt.Println(...)` — writes to stdout ❌
- `log.Print(...)` — writes to stdout by default ❌
- `os.Stdout.Write(...)` — direct stdout write ❌

## Plugin Categories

semrel defines six plugin categories, each with its own gRPC service:

| Category             | Service                  | RPC(s)                          |
|---------------------|--------------------------|---------------------------------|
| CI Condition         | `CIConditionPlugin`      | `VerifyConditions`              |
| Provider             | `ProviderPlugin`         | `GetLastRelease`, `GetCommitsSince`, `CreateRelease`, `UploadAsset` |
| Commit Analyzer      | `CommitAnalyzerPlugin`   | `AnalyzeCommits`                |
| Changelog Generator  | `ChangelogGeneratorPlugin` | `GenerateNotes`              |
| Files Updater        | `FilesUpdaterPlugin`     | `UpdateFiles`                   |
| Hooks                | `HooksPlugin`            | `OnSuccess`, `OnFail`           |

See [`api/proto/v1/semantic_release.proto`](../api/proto/v1/semantic_release.proto) for the full API.

## Writing a Plugin

### 1. Scaffold your plugin repo

A plugin is a standalone Go binary. Minimum structure:

```
my-plugin/
├── main.go
├── plugin.go         # gRPC service implementation
├── go.mod
└── .semrel.yaml      # optional: plugin self-description
```

### 2. Implement the gRPC service

```go
// plugin.go
package main

import (
    "context"
    "os"

    hclog "github.com/hashicorp/go-hclog"
    semrelv1 "github.com/GoSemantics/semrel/api/gen/v1"
)

type MyCommitAnalyzer struct {
    logger hclog.Logger
}

func (a *MyCommitAnalyzer) AnalyzeCommits(
    ctx context.Context,
    req *semrelv1.AnalyzeCommitsRequest,
) (*semrelv1.AnalyzeCommitsResponse, error) {
    a.logger.Info("analyzing commits", "count", len(req.Ctx.Commits))

    // Example: always return patch bump
    return &semrelv1.AnalyzeCommitsResponse{
        Bump:   semrelv1.BumpLevel_BUMP_LEVEL_PATCH,
        Reason: "all commits are patch-level",
    }, nil
}
```

### 3. Wire up go-plugin in main.go

```go
// main.go
package main

import (
    "os"

    "github.com/hashicorp/go-plugin"
    hclog "github.com/hashicorp/go-hclog"
)

func main() {
    logger := hclog.New(&hclog.LoggerOptions{
        Name:   "my-commit-analyzer",
        Output: os.Stderr,   // Always stderr
        Level:  hclog.Debug,
    })

    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: plugin.HandshakeConfig{
            ProtocolVersion:  1,
            MagicCookieKey:   "SEMREL_PLUGIN",
            MagicCookieValue: "semrel",
        },
        Plugins: map[string]plugin.Plugin{
            "commit-analyzer": &CommitAnalyzerGRPCPlugin{
                Impl: &MyCommitAnalyzer{logger: logger},
            },
        },
        GRPCServer: plugin.DefaultGRPCServer,
        Logger:     logger,
    })
}
```

### 4. Reference the plugin in `.semrel.yaml`

```yaml
plugins:
  - uses: my-commit-analyzer
    with:
      some_option: value
```

## Plugin Discovery

semrel discovers plugins at:

```
.semrel/<GOOS>_<GOARCH>/<plugin-name>/<version>/<plugin-binary>
```

Example:
```
.semrel/linux_amd64/my-commit-analyzer/1.0.0/my-commit-analyzer
```

Plugins can also be auto-downloaded from the plugin registry when a `registry_url`
is configured in `.semrel.yaml`.

## CI/CD Notes

- Plugin binaries must be executable (`chmod +x`)
- On Windows, plugin binaries must end in `.exe`
- Air-gapped environments: pre-populate the `.semrel/` directory instead of using auto-download
- Plugins run with the same environment variables as the semrel process

## Testing Your Plugin

To verify your plugin does not write to stdout, add a test:

```go
func TestPluginDoesNotWriteToStdout(t *testing.T) {
    // Redirect stdout to a pipe
    orig := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    // Start plugin (integration test)
    runPluginLogic()

    w.Close()
    os.Stdout = orig
    var buf bytes.Buffer
    io.Copy(&buf, r)

    if buf.Len() > 0 {
        t.Errorf("plugin wrote %d bytes to stdout (forbidden): %q", buf.Len(), buf.String())
    }
}
```

## Related

- [ADR-001: gRPC Plugin Transport](adr/ADR-001-grpc-plugin-transport.md)
- [Plugin proto definition](../api/proto/v1/semantic_release.proto)
- [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin)
- [hashicorp/go-hclog](https://github.com/hashicorp/go-hclog)
