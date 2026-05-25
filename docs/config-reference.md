<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: 2026 The semrel Authors -->

# Configuration Reference

This document describes all supported options in `.semrel.yaml`.

## File Location

semrel looks for `.semrel.yaml` in the current working directory by default.
Use the `--config` flag to specify a different path:

```bash
semrel release --config path/to/.semrel.yaml
```

## Full Example

```yaml
tagPrefix: "v"

branches:
  - name: main
  - name: next
    prerelease: next
  - name: 1.x
    maintenance: true
  - name: release/*

rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: perf
    bump: patch
  - type: revert
    bump: patch
  - type: docs
    bump: patch

plugins:
  - uses: github
  - uses: npm
  - uses: docker
    args:
      image: myorg/myapp
```

## Options

### `tagPrefix`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tagPrefix` | string | `"v"` | Prefix prepended to the version number in git tags. |

```yaml
tagPrefix: "v"         # creates tags like v1.2.3
tagPrefix: ""          # creates tags like 1.2.3
tagPrefix: "release-"  # creates tags like release-1.2.3
```

### `branches`

Defines which branches trigger releases and their behavior.
Only commits on a listed branch will produce a release.
If omitted, all branches are eligible.

```yaml
branches:
  - name: main
  - name: next
    prerelease: next
  - name: 1.x
    maintenance: true
  - name: release/*
```

#### Branch fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | — | Branch name or glob pattern (for example `release/*`) |
| `prerelease` | string | — | Pre-release channel name such as `alpha`, `beta`, or `next` |
| `maintenance` | boolean | `false` | If `true` (or if name matches `N.x` or `N.M.x`), only patch bumps are allowed |

#### Maintenance branch auto-detection

Branches matching the pattern `N.x` or `N.M.x` (for example `1.x`, `1.2.x`, `2.x`) are automatically treated as maintenance branches without setting `maintenance: true`.

### `rules`

Maps conventional commit types to semver bump levels.
If omitted, the default rules apply.

```yaml
rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: perf
    bump: patch
  - type: revert
    bump: patch
```

Breaking changes (`feat!`, `BREAKING CHANGE:` footer) always cause a **major** bump regardless of the rules.

#### Rule fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Conventional commit type (for example `feat`, `fix`, `docs`) |
| `bump` | string | One of `major`, `minor`, `patch` |

#### Default rules

| Commit type | Bump |
|-------------|------|
| `feat` | minor |
| `fix` | patch |
| `perf` | patch |
| `revert` | patch |

### `plugins`

Defines the ordered list of external plugin binaries to run during a release.

- `uses: github` resolves to `semrel-plugin-github`
- semrel first checks `~/.semrel/plugins/`, then falls back to `$PATH`
- `path:` can be used to point at a specific binary
- Install a plugin with `semrel plugin install github`, or place the binary manually in `~/.semrel/plugins/`

```yaml
plugins:
  - uses: github
  - uses: npm
  - uses: docker
    args:
      image: myorg/myapp
```

`args:` values are exposed to the plugin process as `SEMREL_PLUGIN_<KEY>` environment variables. Keys are uppercased and normalized by replacing `-`, `.`, and spaces with `_`.

#### Common standalone plugins

| Name | Type | Binary | Repository |
|------|------|--------|------------|
| `github` | Provider | `semrel-plugin-github` | [SemRels/provider-github](https://github.com/SemRels/provider-github) |
| `gitlab` | Provider | `semrel-plugin-gitlab` | [SemRels/provider-gitlab](https://github.com/SemRels/provider-gitlab) |
| `gitea` | Provider | `semrel-plugin-gitea` | [SemRels/provider-gitea](https://github.com/SemRels/provider-gitea) |
| `bitbucket` | Provider | `semrel-plugin-bitbucket` | [SemRels/provider-bitbucket](https://github.com/SemRels/provider-bitbucket) |
| `npm` | Updater | `semrel-plugin-npm` | [SemRels/updater-npm](https://github.com/SemRels/updater-npm) |
| `docker` | Updater | `semrel-plugin-docker` | [SemRels/updater-docker](https://github.com/SemRels/updater-docker) |
| `helm` | Updater | `semrel-plugin-helm` | [SemRels/updater-helm](https://github.com/SemRels/updater-helm) |
| `cargo` | Updater | `semrel-plugin-cargo` | [SemRels/updater-cargo](https://github.com/SemRels/updater-cargo) |
| `python` | Updater | `semrel-plugin-python` | [SemRels/updater-python](https://github.com/SemRels/updater-python) |
| `gradle` | Updater | `semrel-plugin-gradle` | [SemRels/updater-gradle](https://github.com/SemRels/updater-gradle) |
| `maven` | Updater | `semrel-plugin-maven` | [SemRels/updater-maven](https://github.com/SemRels/updater-maven) |
| `gobinary` | Updater | `semrel-plugin-gobinary` | [SemRels/updater-go](https://github.com/SemRels/updater-go) |
| `nuget` | Updater | `semrel-plugin-nuget` | [SemRels/updater-nuget](https://github.com/SemRels/updater-nuget) |
| `homebrew` | Updater | `semrel-plugin-homebrew` | [SemRels/updater-homebrew](https://github.com/SemRels/updater-homebrew) |
| `terraform` | Updater | `semrel-plugin-terraform` | [SemRels/updater-terraform](https://github.com/SemRels/updater-terraform) |
| `slack` | Hook | `semrel-plugin-slack` | [SemRels/hook-slack](https://github.com/SemRels/hook-slack) |
| `matrix` | Hook | `semrel-plugin-matrix` | [SemRels/hook-matrix](https://github.com/SemRels/hook-matrix) |
| `email` | Hook | `semrel-plugin-email` | [SemRels/hook-email](https://github.com/SemRels/hook-email) |
| `jira` | Hook | `semrel-plugin-jira` | [SemRels/hook-jira](https://github.com/SemRels/hook-jira) |

> **Note:** Git tag creation and `CHANGELOG.md` updates are always performed by the core pipeline. They are not configured as plugins.

#### Plugin fields

| Field | Type | Description |
|-------|------|-------------|
| `uses` | string | Plugin name that resolves to `semrel-plugin-<uses>` |
| `path` | string | Optional explicit path to a plugin binary |
| `args` | map | Plugin-specific arguments, exposed as `SEMREL_PLUGIN_<KEY>` env vars |

#### Local development example

```yaml
plugins:
  - path: ./bin/semrel-plugin-demo
    args:
      endpoint: https://staging.example.com
```

When `path:` is set, semrel skips normal name-based discovery and executes that binary directly.

## CLI Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `.semrel.yaml` | Path to configuration file |
| `--dry-run` | bool | `false` | Simulate release without making changes |
| `--version` | — | — | Print version and exit |

## Subcommands

### `semrel release`

Runs the full release pipeline:
1. Load config
2. Detect current branch and branch config
3. Read git tags to find the last released version
4. Collect commits since the last tag
5. Parse commits using Conventional Commits
6. Calculate next version
7. Generate changelog
8. Create annotated git tag
9. Prepend to `CHANGELOG.md`
10. Execute configured plugin binaries in order

```bash
semrel release
semrel release --dry-run
semrel release --config .semrel.yaml
```

### `semrel plugin install`

Installs a standalone plugin binary into `~/.semrel/plugins/`.

```bash
semrel plugin install github
semrel plugin install npm
```

### `semrel lint`

Validates commit messages since the last tag against Conventional Commits.

```bash
semrel lint
```

Exits with non-zero status if any commits are non-conventional.