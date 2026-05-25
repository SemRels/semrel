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
│  Plugin Runner  │   pkg/builtins + pkg/plugin — run cfg.Plugins after tag/changelog
│  (built-in +   │   Built-ins: github, npm, docker, helm, slack, matrix, gitlab,
│   external)     │   gitea, cargo, python, gradle, maven, gobinary
└─────────────────┘
```

## Package Structure

| Package                 | Responsibility                                                    |
|-------------------------|-------------------------------------------------------------------|
| `cmd/semrel`            | CLI entry point (`main.go`)                                       |
| `internal/cli`          | Cobra commands: `release`, `lint`                                 |
| `internal/version`      | Build-time version constant                                       |
| `internal/registry`     | Plugin registry client — download/cache external plugin binaries  |
| `pkg/config`            | `.semrel.yaml` parsing, branch config, maintenance detection      |
| `pkg/git`               | Git operations: tags, commits, branch name, tag creation          |
| `pkg/commits`           | Conventional Commits parser (including `ParseMulti`)              |
| `pkg/semver`            | Version parsing, bump calculation, pre-release/maintenance support|
| `pkg/releasenotes`      | Structured release notes model + multiple renderers               |
| `pkg/changelog`         | Changelog generator (delegates to `releasenotes`)                 |
| `pkg/plugin`            | `Executor` interface, `Registry`, `ReleaseContext`, `Result`      |
| `pkg/builtins`          | `DefaultRegistry()` — 13 built-in in-process `Executor` plugins   |
| `api/proto/v1`          | Protobuf definitions for the future external plugin gRPC interface |

## Plugin Architecture

semrel supports two plugin modes:

### 1. Built-in In-Process Plugins (current)

Built-in plugins run **in the same process** as semrel, registered via `pkg/builtins.DefaultRegistry()`. They implement the `pkg/plugin.Executor` interface:

```
┌──────────────────────────────────────────┐
│  semrel (release pipeline)               │
│                                          │
│  pkg/builtins.DefaultRegistry()          │
│     ├─ github   (creates GitHub Release) │
│     ├─ npm      (publishes to npm)       │
│     ├─ docker   (tags + pushes image)    │
│     ├─ helm     (updates Chart.yaml)     │
│     ├─ slack    (posts notification)     │
│     ├─ matrix   (posts notification)     │
│     ├─ gitlab   (creates GitLab Release) │
│     ├─ gitea    (creates Gitea Release)  │
│     ├─ cargo    (publishes crate)        │
│     ├─ python   (publishes to PyPI)      │
│     ├─ gradle   (runs gradle publish)    │
│     ├─ maven    (runs mvn deploy)        │
│     └─ gobinary (cross-compiles + zips)  │
└──────────────────────────────────────────┘
```

Configuration in `.semrel.yaml`:
```yaml
plugins:
  - uses: github      # env: GITHUB_TOKEN
  - uses: npm         # env: NPM_TOKEN
  - uses: docker
    args:
      image: myrepo/myapp  # or env: DOCKER_IMAGE
```

### 2. External Out-of-Process Plugins (planned)

Future support for plugins as **separate binaries** communicating over gRPC, using [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin). The `internal/registry` package already supports downloading and caching external plugin binaries from the registry at `https://semrels.github.io/semrel-registry`.

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
└──────────────────────────────────────────┘
```

### Plugin lifecycle

1. `semrel` reads the `plugins:` list from `.semrel.yaml`
2. After git tag (step 10) and CHANGELOG.md update (step 11), plugins execute in order
3. Built-in plugins: resolved from `pkg/builtins.DefaultRegistry()` and executed in-process
4. External plugins (future): resolved from local path or downloaded from registry

### Writing a Plugin

See [docs/plugin-development.md](plugin-development.md) for the `Executor` interface and a step-by-step guide.

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

- All plugins run as in-process executors — built-in plugins are sandboxed by configuration (they only act when the relevant env vars / args are set).
- Plugins communicate only through the defined Protobuf API (external plugins) or the `pkg/plugin.Executor` interface (built-in plugins).
- The plugin registry validates checksums before execution.
- See [SECURITY.md](../SECURITY.md) for the vulnerability disclosure policy.
