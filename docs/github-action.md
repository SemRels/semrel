# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors

# GitHub Action

`SemRels/semrel` ships a composite GitHub Action so you can drop automated
semantic releases into any workflow in seconds.

## Quick start

```yaml
name: Release
on:
  push:
    branches: [main]

permissions:
  contents: write   # required to push tags and create GitHub Releases

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0          # semrel needs full history

      - uses: SemRels/semrel@main
        with:
          dry-run: 'false'
```

## Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `dry-run` | Simulate the release without making changes | `false` |
| `config` | Path to `.semrel.yaml` (relative to `working-directory`) | `.semrel.yaml` |
| `working-directory` | Repository root to release | `.` |
| `semrel-version` | git ref / tag / SHA to pin the semrel version | *(action version)* |
| `go-version-file` | Path to `go.mod` / `go.work` for toolchain selection | *(action's `go.mod`)* |

## Outputs

| Output | Description |
|--------|-------------|
| `version` | New version, e.g. `v1.2.3` (empty if no release was made) |
| `released` | `"true"` if a release was made, `"false"` otherwise |
| `changelog` | Changelog entry generated for this release |

## Using outputs

```yaml
- uses: SemRels/semrel@main
  id: semrel

- name: Create GitHub Release
  if: steps.semrel.outputs.released == 'true'
  uses: softprops/action-gh-release@v2
  with:
    tag_name: ${{ steps.semrel.outputs.version }}
    body: ${{ steps.semrel.outputs.changelog }}
```

## Dry-run mode

```yaml
- uses: SemRels/semrel@main
  with:
    dry-run: 'true'
```

Prints a full preview of what would happen (next version, changelog, git
operations) without writing anything.

## Configuration

Place `.semrel.yaml` at the repository root (or pass `config:` to point
elsewhere). See [docs/config-reference.md](config-reference.md) for all
available options.

## Required permissions

| Permission | Level | Reason |
|-----------|-------|--------|
| `contents` | `write` | Commit release files and push git tags |
| `id-token` | `write` | (optional) OIDC-based Sigstore signing via the cosign plugin |
