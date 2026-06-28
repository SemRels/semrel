<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: 2026 The semrel Authors -->

# Configuration Reference

This document describes all supported options in `.semrel.yaml`.

## JSON Schema & IDE Support

A [JSON Schema](https://registry.semrel.io/schemas/core/v1.json) is published at **`https://registry.semrel.io/schemas/core/v1.json`**.

`semrel config init` adds the schema directive automatically. For existing files, add this comment at the top of your `.semrel.yaml`:

```yaml
# yaml-language-server: $schema=https://registry.semrel.io/schemas/core/v1.json
```

This enables **auto-complete, validation and hover documentation** in:
- **VS Code** — install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml)
- **JetBrains IDEs** (IntelliJ, GoLand, …) — built-in YAML support
- **Neovim / Emacs** — via `yaml-language-server`
- Any other editor with JSON Schema / YAML LSP support

## File Location

semrel automatically discovers config files in the current working directory in this priority order:

1. `.semrel.yaml`
2. `.semrel.yml`
3. `.semrel.toml`
4. `.semrel.json`

Use `--config` to specify a different path explicitly:

```bash
semrel release --config path/to/my-config.toml
```

## Full Example

All three formats are equivalent. Choose whichever fits your project's conventions.

### YAML (`.semrel.yaml`)

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

### TOML (`.semrel.toml`)

```toml
tagPrefix = "v"

[[branches]]
name = "main"

[[branches]]
name = "next"
prerelease = "next"

[[branches]]
name = "1.x"
maintenance = true

[[branches]]
name = "release/*"

[[rules]]
type = "feat"
bump = "minor"

[[rules]]
type = "fix"
bump = "patch"

[[rules]]
type = "perf"
bump = "patch"

[[rules]]
type = "revert"
bump = "patch"

[[rules]]
type = "docs"
bump = "patch"

[[plugins]]
uses = "github"

[[plugins]]
uses = "npm"

[[plugins]]
uses = "docker"

[plugins.args]
image = "myorg/myapp"
```

### JSON (`.semrel.json`)

```json
{
  "tagPrefix": "v",
  "branches": [
    { "name": "main" },
    { "name": "next", "prerelease": "next" },
    { "name": "1.x", "maintenance": true },
    { "name": "release/*" }
  ],
  "rules": [
    { "type": "feat", "bump": "minor" },
    { "type": "fix",  "bump": "patch" },
    { "type": "perf", "bump": "patch" },
    { "type": "revert", "bump": "patch" },
    { "type": "docs", "bump": "patch" }
  ],
  "plugins": [
    { "uses": "github" },
    { "uses": "npm" },
    {
      "uses": "docker",
      "args": { "image": "myorg/myapp" }
    }
  ]
}
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

```toml
tagPrefix = "v"
```

```json
{ "tagPrefix": "v" }
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
- semrel first checks `.semrel/plugins/` (project-local), then `~/.semrel/plugins/` (global), then `$PATH`
- `path:` can be used to point at a specific binary
- Install a plugin with `semrel plugin install github`, or place the binary manually in `~/.semrel/plugins/` and keep pinned versions current with `semrel plugin update`
- During `semrel release`, missing plugins are auto-restored from `.semrel.lock` when present
- Supported plugin categories include providers, analyzers, generators, conditions, hooks, updaters, plus parity-foundation categories packagers and publishers

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
| `packager-nfpm` | Packager | `semrel-plugin-packager-nfpm` | _planned_ |
| `publisher-oci` | Publisher | `semrel-plugin-publisher-oci` | _planned_ |
| `publisher-generic-http` | Publisher | `semrel-plugin-publisher-generic-http` | _planned_ |
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
| `phase` | string | When to run: `condition`, `pre-tag`, or `release` (default: `release`) |

#### Local development example

```yaml
plugins:
  - path: ./bin/semrel-plugin-demo
    args:
      endpoint: https://staging.example.com
```

When `path:` is set, semrel skips normal name-based discovery and executes that binary directly.

### `version_ceiling`

Sets the maximum version semrel is allowed to release. If the computed next version
would exceed this value, semrel applies `ceiling_strategy`.

```yaml
version_ceiling: "2.0.0"
```

Common use cases:
- hold a project on `0.x` until you intentionally allow `1.0.0`
- prevent accidental major releases beyond a policy limit

### `ceiling_strategy`

Controls what semrel does when the computed next version exceeds `version_ceiling`.
Default: `clamp`.

| Value | Description |
|-------|-------------|
| `clamp` | Release at `version_ceiling` instead of the higher computed version |
| `skip` | Skip the release when the next version would exceed the ceiling |
| `error` | Exit with an error and do not release |

```yaml
version_ceiling: "2.0.0"
ceiling_strategy: clamp
```

### `commit_changelog`

Controls whether semrel writes **and** commits `CHANGELOG.md` before creating the git tag.
Default: `true`.

| Value | Behaviour |
|-------|-----------|
| `true` (default) | semrel generates, writes, and commits `CHANGELOG.md` using its built-in changelog generator. |
| `false` | semrel skips the built-in `CHANGELOG.md` write entirely. Use this when a pre-tag generator plugin (e.g. `@semrel/generator-changelog-md`) is responsible for writing the changelog. |

```yaml
commit_changelog: false
```

**Using a changelog generator plugin** — set `commit_changelog: false` so the
plugin's `CHANGELOG.md` is the only version committed:

```yaml
commit_changelog: false

plugins:
  - uses: @semrel/generator-changelog-md
    phase: pre-tag           # runs before the tag, auto-committed by semrel
    args:
      keep_releases: "10"    # required: without this the plugin only outputs to stdout
```

### `tag_exists_strategy`

Controls what semrel does when the computed tag already exists. Default:
`update-changelog`.

| Value | Description |
|-------|-------------|
| `update-changelog` | Update or write `CHANGELOG.md` without creating a new tag |
| `skip` | Skip the release cleanly |
| `error` | Exit with an error |

```yaml
tag_exists_strategy: update-changelog
```

### `workspace`

Enables multi-package workspace (monorepo) mode. When set, `semrel workspace release`
orchestrates releases for all configured packages.

```yaml
workspace:
  strategy: independent   # "independent" (default) or "lockstep"

  # Explicit package list (takes precedence over pattern when both are set)
  packages:
    - path: packages/api
      tagPrefix: "packages/api@v"
    - path: packages/ui
      tagPrefix: "packages/ui@v"
      dependsOn: [packages/api]   # released only after api succeeds

  # OR: auto-discover via glob pattern
  # pattern: "packages/*"

  fail_fast: false   # default: collect all errors
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `strategy` | string | `independent` | `independent` — each package its own semver. `lockstep` — all packages share the same version. |
| `packages` | list | — | Explicit package list. Each entry needs `path`; `tagPrefix` and `dependsOn` are optional. |
| `pattern` | string | — | Glob pattern for auto-discovery (e.g. `packages/*`). Mutually exclusive with `packages`. |
| `fail_fast` | bool | `false` | Stop the workspace release on the first package failure. |

Each package directory can have its own `.semrel.yaml` that overrides the root
configuration for that package (branches, rules, plugins, tagPrefix).

See the [Monorepo guide](https://semrel.io/guide/monorepo/) for full details and CI examples.

## CLI Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | auto-detect | Path to configuration file (`.semrel.yaml`, `.semrel.yml`, `.semrel.toml`, or `.semrel.json`) |
| `--dry-run` | bool | `false` | Simulate release without making changes |
| `--env-file` | string | `.env` | Path to `.env` file to load before running the command |
| `--no-color` | bool | `false` | Disable coloured terminal output |
| `-o, --output` | string | `text` | Output format: `text` or `json` |
| `--version` | — | — | Print version and exit |

## Subcommands

### `semrel release`

Runs the full release pipeline:
1. Load and validate `.semrel.yaml`
2. Run condition-phase plugins (gates)
3. Check current branch is configured for release
4. Find last tag, collect commits since then
5. Parse commits against Conventional Commits rules
6. Calculate next SemVer bump
7. Generate changelog / release notes
8. Run pre-tag plugins (for example version file updaters)
9. Commit `CHANGELOG.md` (unless `commit_changelog: false`)
10. Create and push git tag
11. Run release-phase plugins (providers, hooks, publishers)

Release-specific flags:

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Preview the release without making changes |
| `--edit` | bool | Open generated release notes in `$EDITOR` before tagging |
| `--interactive` | bool | Pause for confirmation before tagging |
| `--github-output` | bool | Write release metadata to `$GITHUB_OUTPUT` |
| `--gitlab-dotenv` | string | Write release metadata as a GitLab dotenv artifact |
| `--output-file` | string | Write release metadata to a file (`.json` or `.env`) |
| `--force-bump-patch-version` | bool | Force a patch release even when no releasable commits are found |

```bash
semrel release
semrel release --dry-run
semrel release --config .semrel.yaml
semrel release --edit
```

### `semrel changelog`

Generate a changelog from unreleased commits without creating a release.

Flags:

| Flag | Type | Description |
|------|------|-------------|
| `--write` | bool | Prepend the generated entry to `CHANGELOG.md` |
| `--since` | string | Start from a specific tag or ref instead of auto-detecting the last tag |

```bash
semrel changelog
semrel changelog --write
semrel changelog --since v1.2.0
```

### `semrel lint`

Validates commit messages since the last tag against Conventional Commits.

```bash
semrel lint
semrel lint --output json
```

### `semrel commitlint`

Validates commit messages against Conventional Commits. Without arguments, it
defaults to the commits since the last tag, matching `semrel lint`.

Flags:

| Flag | Type | Description |
|------|------|-------------|
| `--from` | string | Start ref (exclusive) for a commit range |
| `--to` | string | End ref (inclusive) for a commit range |
| `--stdin` | bool | Read a single commit message from stdin |

```bash
semrel commitlint
semrel commitlint --from HEAD~5 --to HEAD
echo "fix: typo" | semrel commitlint --stdin
```

### `semrel doctor`

Runs pre-flight checks for configuration, plugins, git state, branch matching,
and common environment variables before a release.

Flags:

| Flag | Type | Description |
|------|------|-------------|
| `--online` | bool | Also ping the semrel registry for plugin availability |

```bash
semrel doctor
semrel doctor --online
```

### `semrel config`

Manages the semrel configuration file.

#### `semrel config init`

Create a new `.semrel.yaml`, either interactively or with defaults.

```bash
semrel config init
semrel config init --no-interactive
semrel config init --force
```

#### `semrel config show`

Print the resolved configuration.

```bash
semrel config show
semrel config show --output json
```

#### `semrel config validate`

Validate the current configuration file.

```bash
semrel config validate
```

#### `semrel config set`

Update a top-level or nested config key in `.semrel.yaml`.

```bash
semrel config set tagPrefix v
semrel config set version_ceiling 2.0.0
semrel config set commit_changelog false
```

### `semrel migrate`

Upgrade `.semrel.yaml` from an older schema version to the current schema.

Flags:

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Show pending migrations without writing files |
| `--no-backup` | bool | Skip writing a timestamped backup before migration |

```bash
semrel migrate
semrel migrate --dry-run
semrel migrate --no-backup
```

### `semrel plugin`

Manage semrel plugins from the registry.

#### `semrel plugin list`

List all available plugins.

```bash
semrel plugin list
```

#### `semrel plugin search`

Search plugins by name, description, or tag.

```bash
semrel plugin search github
```

#### `semrel plugin install`

Installs a standalone plugin binary into `.semrel/plugins/` by default.

```bash
semrel plugin install github
semrel plugin install npm
```

#### `semrel plugin update`

Checks for newer plugin releases from the registry and updates pinned plugins.

Without arguments, semrel uses `.semrel.lock` and updates all pinned plugins.

```bash
semrel plugin update --check
semrel plugin update
semrel plugin update @semrel/github
```

#### `semrel plugin restore`

Reinstalls exact plugin versions pinned in `.semrel.lock`. Useful in CI and on fresh clones.

```bash
semrel plugin restore
```

### `semrel update`

Check for a newer semrel release and install it in place.

Flags:

| Flag | Type | Description |
|------|------|-------------|
| `--check` | bool | Only check for a newer version; do not download it |

```bash
semrel update
semrel update --check
```