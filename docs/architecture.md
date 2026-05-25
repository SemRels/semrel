<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: 2026 The semrel Authors -->

# Architecture Overview

This document describes the high-level architecture of semrel, how the release pipeline works, and how the plugin system is designed.

## Pipeline Overview

```
.semrel.yaml
     │
     ▼
┌─────────────┐
│  CLI (cobra) │   semrel release [--dry-run]
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  Config Loader  │   pkg/config — load .semrel.yaml, apply defaults
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  Git Repository │   pkg/git — read tags, commits, current branch
└──────┬──────────┘
       │ raw commit messages
       ▼
┌─────────────────┐
│  Commit Parser  │   pkg/commits — Conventional Commits spec
└──────┬──────────┘
       │ []Commit{Type, Scope, Breaking, …}
       ▼
┌─────────────────┐
│ SemVer Calculator│  pkg/semver — compute next version
│  (bump rules)   │   respects maintenance, prerelease, breaking
└──────┬──────────┘
       │ nextVersion
       ▼
┌─────────────────┐
│ Release Notes   │   pkg/releasenotes — structured intermediate model
│   Builder       │   Breaking / Features / Fixes / Others sections
└──────┬──────────┘
       │ *ReleaseNotes
       ▼
┌─────────────────┐
│  Renderers      │   RenderMarkdown() — CHANGELOG.md
│                 │   RenderKeepAChangelog() — KaC 1.0.0 spec
│                 │   RenderText() — notification excerpts
└──────┬──────────┘
       │ formatted output
       ▼
┌─────────────────┐
│  Plugin Runner  │   pkg/plugin — load + execute plugins via gRPC
│  (git, gh, …)  │   Creates tag, publishes release, updates files
└─────────────────┘
```

## Package Structure

| Package                 | Responsibility                                                    |
|-------------------------|-------------------------------------------------------------------|
| `cmd/semrel`            | CLI entry point (`main.go`)                                       |
| `internal/cli`          | Cobra commands: `release`, `lint`                                 |
| `internal/version`      | Build-time version constant                                       |
| `internal/registry`     | Plugin registry client — download/cache plugins                  |
| `pkg/config`            | `.semrel.yaml` parsing, branch config, maintenance detection      |
| `pkg/git`               | Git operations: tags, commits, branch name, tag creation          |
| `pkg/commits`           | Conventional Commits parser (including `ParseMulti`)              |
| `pkg/semver`            | Version parsing, bump calculation, pre-release/maintenance support|
| `pkg/releasenotes`      | Structured release notes model + multiple renderers               |
| `pkg/changelog`         | Changelog generator (delegates to `releasenotes`)                 |
| `pkg/plugin`            | Plugin loader and executor (hashicorp/go-plugin + gRPC)           |
| `api/proto/v1`          | Protobuf definitions for the plugin gRPC interface                |

## Plugin Architecture

Plugins are **out-of-process** binaries that communicate with the semrel core over gRPC, using [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) as the transport layer.

```
┌──────────────────────────────────────────┐
│  semrel (host process)                   │
│                                          │
│  pkg/plugin.Loader                       │
│     └─ launches plugin binary as child  │
│         via go-plugin + gRPC             │
└──────────────────────────┬───────────────┘
                           │  gRPC (unix socket / TCP)
                           │
┌──────────────────────────▼───────────────┐
│  Plugin binary (child process)           │
│                                          │
│  Implements api/proto/v1.ReleasePlugin   │
│  gRPC service                            │
│    • PublishRelease(ReleaseRequest)      │
│    • GetMetadata() → PluginInfo          │
└──────────────────────────────────────────┘
```

### Plugin lifecycle

1. `semrel` reads the `plugins:` list from `.semrel.yaml`
2. For each plugin, `pkg/plugin.Loader` resolves the binary path (local `path:` or downloaded from registry)
3. The binary is launched as a child process via `go-plugin`
4. `semrel` calls the gRPC `PublishRelease` method with a `ReleaseRequest` (version, changelog, tag, artifacts)
5. The plugin performs its action (create GitHub Release, push Docker image, etc.) and returns a `ReleaseResponse`
6. The child process exits cleanly

### Writing a Plugin

See [docs/plugin-development.md](plugin-development.md) for a step-by-step guide.

## Branch Strategy

```
main ────────────────────────────────────► stable releases (v1.0.0, v1.1.0, …)
  │
  ├── next ──────────────────────────────► pre-release channel (v1.2.0-next.1, …)
  │
  ├── feat/* ────────────────────────────► feature branches (short-lived)
  │
  └── 1.x ─────────────────────────────► maintenance branch (v1.0.1, v1.0.2, …)
       └── 1.1.x ──────────────────────► sub-maintenance branch
```

- **main**: stable releases only. All status checks required.
- **Pre-release branches**: mapped to a channel via `prerelease:` in config. Version increments are `N.M.P-channel.K`.
- **Maintenance branches** (`N.x`, `N.M.x`): patch-only bumps. Cannot produce minor or major versions.

## Security Model

- All plugins run in separate processes — a plugin cannot affect the host process memory directly.
- Plugins communicate only through the defined Protobuf API.
- The plugin registry validates checksums before execution.
- See [SECURITY.md](../SECURITY.md) for the vulnerability disclosure policy.
