<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: 2026 The semrel Authors -->

# Plugin Development Guide

This guide explains how to write a semrel plugin using the `pkg/plugin.Executor`
interface. Plugins execute **in-process** as part of the semrel release pipeline.

## Plugin Interface

All plugins implement `pkg/plugin.Executor`:

```go
// Executor is the in-process plugin interface.
type Executor interface {
    Name()    string    // unique name used in .semrel.yaml `uses:`
    Version() string    // plugin version string (e.g. "1.0.0")
    Validate() error    // validate config without side-effects
    Execute(ctx context.Context, rel ReleaseContext) (*Result, error)
}
```

### ReleaseContext

`Execute` receives all release information:

```go
type ReleaseContext struct {
    Version         string            // new version, e.g. "1.2.3"
    PreviousVersion string            // previous version
    TagName         string            // git tag name, e.g. "v1.2.3"
    Repository      string            // "owner/repo"
    Changelog       string            // generated changelog text
    CommitSHA       string            // HEAD commit SHA
    IsPrerelease    bool
    IsDryRun        bool
    Metadata        map[string]string // args from .semrel.yaml
}
```

### Result

Return a `*Result` from `Execute`:

```go
type Result struct {
    Name       string
    Outputs    map[string]string // key-value pairs, e.g. {"release_url": "..."}
    Skipped    bool
    SkipReason string
}

// Helpers
plugin.SuccessResult(name, outputs)       // success with outputs
plugin.SkippedResult(name, reason)        // graceful skip
```

## Writing a Built-in Style Plugin

Use `plugin.BasePlugin` to get default `Validate()` and version boilerplate:

```go
package myplugin

import (
    "context"
    "fmt"
    "os"

    "github.com/GoSemantics/semrel/pkg/plugin"
)

type MyPlugin struct{ plugin.BasePlugin }

func New() *MyPlugin {
    return &MyPlugin{plugin.NewBasePlugin("myplugin", "1.0.0")}
}

func (p *MyPlugin) Validate() error {
    if os.Getenv("MY_TOKEN") == "" {
        return plugin.ErrInvalidConfig{
            Plugin:  p.Name(),
            Message: "MY_TOKEN env var is required",
        }
    }
    return nil
}

func (p *MyPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
    // Always respect dry-run
    if rel.IsDryRun {
        return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
    }

    token := os.Getenv("MY_TOKEN")
    // ... do the actual work ...

    return plugin.SuccessResult(p.Name(), map[string]string{
        "version": rel.Version,
        "url":     fmt.Sprintf("https://example.com/releases/%s", rel.TagName),
    }), nil
}
```

## Registering Your Plugin

Add it to `pkg/builtins/registry.go` `allBuiltins()` to make it a core built-in,
or register it dynamically:

```go
reg := plugin.NewRegistry()
_ = reg.Register(myplugin.New())
result, err := reg.Execute("myplugin", ctx, relCtx)
```

## Configuration in `.semrel.yaml`

```yaml
plugins:
  - uses: myplugin
    args:
      endpoint: https://custom.example.com   # available as rel.Metadata["endpoint"]
```

`args` values are passed as `rel.Metadata` (all values converted to `string`).

## Design Rules

| Rule | Reason |
|------|--------|
| **Always handle `IsDryRun`** — return `SuccessResult` with `dry_run=true` | Dry-run must never have side-effects |
| **Return `SkippedResult` when misconfigured** (optional config) | Graceful degradation vs. hard errors |
| **Return error for required config** (token missing) | Fail fast before any mutations |
| **Read config from env vars** with `args:` overrides | Standard semrel convention |
| **All logging to `os.Stderr`** | stdout is reserved for `--output-format json` |

## Testing Your Plugin

```go
func TestMyPlugin_DryRun(t *testing.T) {
    p := myplugin.New()
    result, err := p.Execute(context.Background(), plugin.ReleaseContext{
        Version:  "1.2.3",
        TagName:  "v1.2.3",
        IsDryRun: true,
    })
    if err != nil {
        t.Fatal(err)
    }
    if result.Outputs["dry_run"] != "true" {
        t.Error("expected dry_run=true")
    }
}

func TestMyPlugin_SkipWhenNotConfigured(t *testing.T) {
    p := myplugin.New()
    // No env vars set
    result, err := p.Execute(context.Background(), plugin.ReleaseContext{Version: "1.0.0"})
    if err != nil {
        t.Fatal(err)
    }
    if !result.Skipped {
        t.Error("expected plugin to be skipped when not configured")
    }
}
```

See `pkg/builtins/registry_test.go` for a complete test example covering all built-ins.

## External Plugins (Planned)

Future versions of semrel will support **out-of-process external plugins** as
standalone binaries, discoverable via the plugin registry at
`https://semrels.github.io/semrel-registry`. These will communicate over gRPC
using `api/proto/v1`.

For now, all custom plugins must be registered in-process via `pkg/plugin.Registry`.

## Related

- [`pkg/plugin/plugin.go`](../pkg/plugin/plugin.go) — `Executor` interface + `Registry`
- [`pkg/builtins/registry.go`](../pkg/builtins/registry.go) — 13 built-in plugin implementations
- [Configuration Reference](config-reference.md) — `plugins:` config section
- [ADR-001: gRPC Plugin Transport](adr/ADR-001-grpc-plugin-transport.md) — future external plugin transport