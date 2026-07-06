<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: 2026 The semrel Authors -->

# Plugin Development Guide

semrel plugins are standalone executables. The core `semrel` binary discovers a plugin binary, starts it as a subprocess, and passes release context through environment variables.

## Start here

- Use [SemRels/plugin-template](https://github.com/SemRels/plugin-template) as the starting point for a new plugin.
- See the standalone plugin repositories under [SemRels](https://github.com/SemRels) for reference implementations.
- Plugins can be written in any language that can read environment variables and exit with the correct status code.

## Execution model

A config entry such as:

```yaml
plugins:
  - uses: github
```

resolves to a binary named `semrel-plugin-github`.

semrel resolves plugin binaries in this order:

1. `path:` from `.semrel.yaml`
2. `.semrel/plugins/semrel-plugin-<name>`
3. `.semrel/plugins/semrel-plugin-<name> (project-local) or ~/.semrel/plugins/semrel-plugin-<name> (global)`
4. `semrel-plugin-<name>` in `$PATH`

When a plugin is invoked, semrel:

1. Creates a child process for the plugin binary
2. Passes release context as environment variables
3. Adds plugin-specific config from `args:` as `SEMREL_PLUGIN_<KEY>` variables
4. Treats exit code `0` as success and any non-zero exit code as failure

## Environment variables

### Release context

| Variable | Description |
|----------|-------------|
| `SEMREL_VERSION` | New version string (e.g. `1.2.3`) |
| `SEMREL_NEXT_VERSION` | Same as `SEMREL_VERSION` (alias) |
| `SEMREL_TAG_NAME` | Full tag name including prefix (e.g. `v1.2.3`) |
| `SEMREL_CURRENT_VERSION` | Previous version before this release |
| `SEMREL_BUMP` | Bump type: `major`, `minor`, or `patch` |
| `SEMREL_BRANCH` | Current branch name |
| `SEMREL_TAG_PREFIX` | Tag prefix configured in `.semrel.yaml` |
| `SEMREL_CHANGELOG` | Generated release changelog |
| `SEMREL_DRY_RUN` | `true` or `false` |
| `SEMREL_COMMITS` | JSON array of raw commit messages since last release (e.g. `["feat: foo","fix: bar"]`) |
| `SEMREL_CONTRIBUTORS` | JSON array of per-release contributor metadata sorted by commit count descending (e.g. `[{"name":"Jane Doe","email":"jane@example.com","commits":3,"firstContribution":true}]`) |
| `SEMREL_REPOSITORY_URL` | Repository base URL (e.g. `https://github.com/org/repo`) — used for PR/commit linkification |

`SEMREL_CONTRIBUTORS` is populated for plugins that run after semrel has analysed the release commit range (for example generator, pre-tag, provider, hook, publisher, and updater plugins). Condition and analyzer plugins should not rely on it.

### Plugin config from `args:`

Each key in the plugin's `args:` block becomes `SEMREL_PLUGIN_<KEY>`.

```yaml
plugins:
  - uses: docker
    args:
      image: myorg/myapp
      registry-url: ghcr.io
```

The plugin process receives:

- `SEMREL_PLUGIN_IMAGE=myorg/myapp`
- `SEMREL_PLUGIN_REGISTRY_URL=ghcr.io`

Key normalization rules:

- Convert to uppercase
- Replace `-` with `_`
- Replace `.` with `_`
- Replace spaces with `_`

## Minimal plugin contract

Your plugin binary should:

1. Read the required `SEMREL_*` environment variables
2. Perform its work
3. Exit with `0` on success
4. Exit with a non-zero code on failure

A typical flow is:

```text
read env vars -> validate config -> respect dry-run -> perform action -> exit 0
```

## Example

This shell-style pseudocode shows the contract:

```bash
#!/usr/bin/env bash
set -euo pipefail

version="${SEMREL_VERSION}"
tag="${SEMREL_TAG_NAME}"
dry_run="${SEMREL_DRY_RUN}"
image="${SEMREL_PLUGIN_IMAGE:-}"

if [ -z "$image" ]; then
  echo "SEMREL_PLUGIN_IMAGE is required" >&2
  exit 1
fi

if [ "$dry_run" = "true" ]; then
  echo "[dry-run] would publish $image:$version"
  exit 0
fi

# perform the real publish step here

echo "published $image:$tag"
exit 0
```

## Local development workflow

Build your plugin binary locally and point `path:` at it while iterating:

```yaml
plugins:
  - path: ./bin/semrel-plugin-demo
    args:
      endpoint: https://staging.example.com
```

You can also install it into the standard project-local location:

```bash
semrel plugin install @semrel/demo
```

Use the full `@namespace/name` reference when the registry entry belongs to a namespace. A bare name such as `demo` only works for namespace-less plugins.

To exercise the normal resolution order without `path:`, place the binary in one of these locations:

- Project-local: `.semrel/plugins/semrel-plugin-demo`
- Project-local: `.semrel/plugins/semrel-plugin-demo`
- User-global (Windows): `~/.semrel/plugins/semrel-plugin-demo.exe` or `.cmd`
- Any directory on `$PATH`

## Plugin Lock File

Project-local installs update `.semrel.lock` in the repository root. The lock file records the canonical plugin reference, the pinned version, and published checksums for every supported platform so the same plugin binaries can be restored on any machine.

Use it as part of the normal team workflow:

1. Install or update a plugin with `semrel plugin install @semrel/demo`
2. Check and apply newer plugin releases with `semrel plugin update --check` / `semrel plugin update`
3. Commit `.semrel.lock` alongside `.semrel.yaml`
4. Run `semrel plugin restore` in CI or after cloning the repository

Installs performed with `--plugin-dir` do not update `.semrel.lock`.

## Reference implementations

The standalone SemRels plugin repositories are the reference implementation for the current architecture, including providers, updaters, hooks, and upcoming parity categories such as packagers and publishers.

Examples:

- [SemRels/provider-github](https://github.com/SemRels/provider-github)
- [SemRels/provider-gitlab](https://github.com/SemRels/provider-gitlab)
- [SemRels/updater-npm](https://github.com/SemRels/updater-npm)
- [SemRels/updater-docker](https://github.com/SemRels/updater-docker)
- [SemRels/hook-slack](https://github.com/SemRels/hook-slack)
- [SemRels/hook-jira](https://github.com/SemRels/hook-jira)

## Design guidelines

| Guideline | Why it matters |
|-----------|----------------|
| Respect `SEMREL_DRY_RUN` | Dry-run must not have side effects |
| Fail fast for missing required config | Misconfiguration should be obvious |
| Keep logs human-readable | Plugin output is streamed directly to the user |
| Treat `args:` as plugin-owned config | Keeps the core binary generic |
| Publish from a standalone repo | Lets plugins version and release independently |

## Related

- [Architecture Overview](architecture.md) — plugin execution flow and protocol
- [Configuration Reference](config-reference.md) — `plugins:` config section
- [SemRels/plugin-template](https://github.com/SemRels/plugin-template) — recommended starting point