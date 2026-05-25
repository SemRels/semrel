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
  - uses: git
  - uses: changelog
  - uses: github
    args:
      repo: SemRels/semrel
```

## Options

### `tagPrefix`

| Field       | Type   | Default | Description                                      |
|-------------|--------|---------|--------------------------------------------------|
| `tagPrefix` | string | `"v"`   | Prefix prepended to the version number in git tags. |

```yaml
tagPrefix: "v"       # creates tags like v1.2.3
tagPrefix: ""        # creates tags like 1.2.3
tagPrefix: "release-" # creates tags like release-1.2.3
```

### `branches`

Defines which branches trigger releases and their behavior.
Only commits on a listed branch will produce a release.
If omitted, all branches are eligible.

```yaml
branches:
  - name: main              # Stable releases
  - name: next              # Pre-release channel "next"
    prerelease: next
  - name: 1.x               # Maintenance — patch bumps only
    maintenance: true
  - name: release/*         # Wildcard pattern
```

#### Branch fields

| Field         | Type    | Default | Description                                           |
|---------------|---------|---------|-------------------------------------------------------|
| `name`        | string  | —       | Branch name or glob pattern (e.g. `release/*`)        |
| `prerelease`  | string  | —       | Pre-release channel name (e.g. `alpha`, `beta`, `next`). Versions will be `1.3.0-beta.1`, `1.3.0-beta.2`, … |
| `maintenance` | boolean | `false` | If `true` (or if name matches `N.x`/`N.M.x`), only patch bumps are allowed. |

#### Maintenance branch auto-detection

Branches matching the pattern `N.x` or `N.M.x` (e.g. `1.x`, `1.2.x`, `2.x`)
are automatically treated as maintenance branches without setting `maintenance: true`.

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

Breaking changes (`feat!`, `BREAKING CHANGE:` footer) always cause a **major** bump
regardless of the rules.

#### Rule fields

| Field  | Type   | Description                                          |
|--------|--------|------------------------------------------------------|
| `type` | string | Conventional commit type (e.g. `feat`, `fix`, `docs`) |
| `bump` | string | One of `major`, `minor`, `patch`                    |

#### Default rules

| Commit type | Bump  |
|-------------|-------|
| `feat`      | minor |
| `fix`       | patch |
| `perf`      | patch |
| `revert`    | patch |

### `plugins`

Defines the ordered list of plugins to run during a release.

```yaml
plugins:
  - uses: git                  # Built-in: create and push git tag
  - uses: changelog            # Built-in: update CHANGELOG.md
  - uses: github               # Built-in: create GitHub Release
    args:
      repo: owner/repo
  - uses: custom-plugin        # External plugin binary
    path: ./bin/custom-plugin
    args:
      key: value
```

#### Plugin fields

| Field  | Type   | Description                                                          |
|--------|--------|----------------------------------------------------------------------|
| `uses` | string | Plugin name (built-in) or identifier                                 |
| `path` | string | Path to plugin binary (for external plugins)                         |
| `args` | map    | Plugin-specific arguments (string keys, any value)                   |

## CLI Flags

| Flag        | Type   | Default          | Description                              |
|-------------|--------|------------------|------------------------------------------|
| `--config`  | string | `.semrel.yaml`   | Path to configuration file               |
| `--dry-run` | bool   | `false`          | Simulate release without making changes  |
| `--version` | —      | —                | Print version and exit                   |

## Subcommands

### `semrel release`

Runs the full release pipeline:
1. Load config
2. Detect current branch and branch config
3. Read git tags to find the last released version
4. Collect commits since the last tag
5. Parse commits using Conventional Commits
6. Calculate next version (respecting maintenance/prerelease settings)
7. Generate changelog
8. Create annotated git tag
9. Prepend to CHANGELOG.md

```bash
semrel release
semrel release --dry-run      # Preview without making changes
semrel release --config .semrel.yaml
```

### `semrel lint`

Validates commit messages since the last tag against Conventional Commits.

```bash
semrel lint
```

Exits with non-zero status if any commits are non-conventional.
