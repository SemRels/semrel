<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: 2026 The semrel Authors -->

# Architecture Overview

This document describes the high-level architecture of semrel, how the release pipeline works, and how plugins are executed after the migration to standalone plugin repositories.

## Pipeline Overview

```
.semrel.yaml
     │
     ▼
┌──────────────┐
│ CLI (cobra)  │   semrel release [--dry-run]
└──────┬───────┘
       │
       ▼
┌─────────────────┐
│ Config Loader   │   pkg/config — load .semrel.yaml, apply defaults
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│ Git Repository  │   pkg/git — read tags, commits, current branch
└──────┬──────────┘
       │ raw commit messages
       ▼
┌─────────────────┐
│ Commit Parser   │   pkg/commits — Conventional Commits spec
└──────┬──────────┘
       │ []Commit{Type, Scope, Breaking, …}
       ▼
┌─────────────────┐
│ SemVer Calculator│  pkg/semver — compute next version
│ (bump rules)    │   respects maintenance, prerelease, breaking
└──────┬──────────┘
       │ nextVersion
       ▼
┌─────────────────┐
│ Release Notes   │   pkg/releasenotes — structured intermediate model
│ Builder         │   Breaking / Features / Fixes / Others sections
└──────┬──────────┘
       │ rendered changelog
       ▼
┌───────────────────────────────┐
│ Plugin Orchestrator           │   pkg/plugininstance.Orchestrator
│ + subprocess runner           │   internal/cli/root.go
└──────┬────────────────────────┘
       │ resolves semrel-plugin-<name>
       ▼
┌───────────────────────────────┐
│ Plugin binary                 │   ~/.semrel/plugins/ or $PATH
│ (standalone child process)    │   semrel-plugin-github, semrel-plugin-npm, …
└───────────────────────────────┘
```

## Package Structure

| Package | Responsibility |
|---------|----------------|
| `cmd/semrel` | CLI entry point (`main.go`) |
| `internal/cli` | Cobra commands and subprocess plugin execution |
| `internal/version` | Build-time version constant |
| `internal/registry` | Plugin registry client and download/cache support |
| `pkg/config` | `.semrel.yaml` parsing, branch config, maintenance detection |
| `pkg/git` | Git operations: tags, commits, branch name, tag creation |
| `pkg/commits` | Conventional Commits parser |
| `pkg/semver` | Version parsing, bump calculation, pre-release handling |
| `pkg/releasenotes` | Structured release notes model and renderers |
| `pkg/changelog` | Changelog generator |
| `pkg/plugin` | Legacy in-process plugin types kept for compatibility |
| `pkg/plugininstance` | Ordered plugin instance orchestration |
| `pkg/builtins` | Minimal empty registry; no plugin implementations are bundled |

## Plugin Architecture

semrel no longer ships plugin implementations in the core binary.

- `pkg/builtins` is intentionally minimal and returns an empty registry.
- Every configured plugin is resolved as an external binary.
- `uses: github` means semrel looks for a binary named `semrel-plugin-github`.
- Plugin binaries are resolved in this order:
  1. `path:` from `.semrel.yaml` (if provided)
  2. `~/.semrel/plugins/semrel-plugin-<name>`
  3. `semrel-plugin-<name>` in `$PATH`

### Execution flow

```
semrel CLI
   │
   ▼
pkg/plugininstance.Orchestrator
   │
   ▼
subprocess runner (internal/cli/root.go)
   │
   ▼
semrel-plugin-<name>
```

### Configuration example

```yaml
plugins:
  - uses: github
  - uses: npm
  - uses: docker
    args:
      image: myorg/myapp
```

Each entry becomes a `plugininstance.PluginSpec`. The orchestrator executes plugin instances in declaration order, so the same plugin binary can be used multiple times with different `args:` blocks.

## Available standalone plugins

Core semrel plugins now live in standalone repositories under [`SemRels`](https://github.com/SemRels):

| Plugin | Type | Repository |
|--------|------|------------|
| `github` | Provider | [SemRels/provider-github](https://github.com/SemRels/provider-github) |
| `gitlab` | Provider | [SemRels/provider-gitlab](https://github.com/SemRels/provider-gitlab) |
| `gitea` | Provider | [SemRels/provider-gitea](https://github.com/SemRels/provider-gitea) |
| `bitbucket` | Provider | [SemRels/provider-bitbucket](https://github.com/SemRels/provider-bitbucket) |
| `npm` | Updater | [SemRels/updater-npm](https://github.com/SemRels/updater-npm) |
| `docker` | Updater | [SemRels/updater-docker](https://github.com/SemRels/updater-docker) |
| `helm` | Updater | [SemRels/updater-helm](https://github.com/SemRels/updater-helm) |
| `cargo` | Updater | [SemRels/updater-cargo](https://github.com/SemRels/updater-cargo) |
| `python` | Updater | [SemRels/updater-python](https://github.com/SemRels/updater-python) |
| `gradle` | Updater | [SemRels/updater-gradle](https://github.com/SemRels/updater-gradle) |
| `maven` | Updater | [SemRels/updater-maven](https://github.com/SemRels/updater-maven) |
| `gobinary` | Updater | [SemRels/updater-go](https://github.com/SemRels/updater-go) |
| `nuget` | Updater | [SemRels/updater-nuget](https://github.com/SemRels/updater-nuget) |
| `homebrew` | Updater | [SemRels/updater-homebrew](https://github.com/SemRels/updater-homebrew) |
| `terraform` | Updater | [SemRels/updater-terraform](https://github.com/SemRels/updater-terraform) |
| `slack` | Hook | [SemRels/hook-slack](https://github.com/SemRels/hook-slack) |
| `matrix` | Hook | [SemRels/hook-matrix](https://github.com/SemRels/hook-matrix) |
| `email` | Hook | [SemRels/hook-email](https://github.com/SemRels/hook-email) |
| `jira` | Hook | [SemRels/hook-jira](https://github.com/SemRels/hook-jira) |
| `gitplugin` | Hook | [SemRels/hook-gitplugin](https://github.com/SemRels/hook-gitplugin) |

## Plugin Binary Protocol

semrel passes release context to plugin binaries through environment variables.

### Core release context

| Environment variable | Meaning |
|----------------------|---------|
| `SEMREL_VERSION` | New version string |
| `SEMREL_TAG_NAME` | Full tag name, including any configured prefix |
| `SEMREL_CHANGELOG` | Generated release changelog |
| `SEMREL_REPOSITORY` | Repository slug such as `owner/repo` |
| `SEMREL_DRY_RUN` | `"true"` or `"false"` |
| `SEMREL_COMMITS` | JSON array of raw commit messages in the current release window |
| `SEMREL_CONTRIBUTORS` | JSON array of contributors for the current release window, including `name`, `email`, `commits`, and `firstContribution` |
| `SEMREL_IS_PRERELEASE` | `"true"` or `"false"` |

### Plugin arguments

Each key in a plugin's `args:` block is converted into an environment variable prefixed with `SEMREL_PLUGIN_`.

```yaml
plugins:
  - uses: docker
    args:
      image: myorg/myapp
      registry-url: ghcr.io
```

This produces:

- `SEMREL_PLUGIN_IMAGE=myorg/myapp`
- `SEMREL_PLUGIN_REGISTRY_URL=ghcr.io`

Key normalization is uppercase with `-`, `.`, and spaces converted to `_`.

### Exit codes

- Exit code `0` = success
- Any non-zero exit code = failure

The core process streams the plugin's stdout and stderr directly, so plugin logs remain visible in normal semrel output.

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

- Plugins are external executables and run with the same OS-level permissions as the `semrel` process.
- The core binary only executes binaries that are explicitly configured and resolvable from `path:`, `~/.semrel/plugins/`, or `$PATH`.
- Install plugins from trusted SemRels repositories or other trusted sources.
- See [SECURITY.md](../SECURITY.md) for the vulnerability disclosure policy.