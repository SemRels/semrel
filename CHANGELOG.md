## v0.12.1 (2026-06-11)

### Bug Fixes

* **doctor:** use @semrel/ namespace in suggestions and config init

## v0.12.0 (2026-06-11)

### Features

* **release:** auto-restore plugins from .semrel.lock before release

### Bug Fixes

* **schema:** update all references to canonical registry URL

## v0.11.1 (2026-06-11)

### Bug Fixes

* **plugin:** standardize namespace to @semrel in documentation and code

## v0.11.0 (2026-06-11)

### Features

* **plugin:** add .semrel.lock for reproducible plugin installs

### Bug Fixes

* **plugin:** enforce namespace for install command
* **plugin:** align registry names, project-local plugin dir, auto-install

## v0.10.2 (2026-06-11)

### Bug Fixes

* **doctor:** use registry plugin names and correct token env vars

## v0.10.1 (2026-06-10)

### Bug Fixes

* **plugin:** show namespaces in plugin list, search and install output

## v0.10.0 (2026-06-10)

### Features

* **doctor:** add plugin recommendations based on project context
* **plugin:** track installs in registry and add --sort downloads to plugin list

## v0.9.1 (2026-06-10)

### Bug Fixes

* **update:** remove Zone.Identifier ADS after binary swap on Windows

### Other Changes

* update README and config reference for v0.9.0

## v0.9.0 (2026-06-10)

### Features

* **cli:** add 'semrel update' self-update command

## v0.8.2 (2026-06-10)

### Bug Fixes

* **registry:** update DefaultBaseURL to custom domain registry.semrel.io
* **commitlint:** default to commits since last tag when no arguments given
* **cli:** show embedded module version and improve config help

### Other Changes

* **config:** reorganize .semrel.yaml structure for clarity
* **cli:** expand help text for all commands

## v0.8.1 (2026-06-10)

### Bug Fixes

* rename module path from GoSemantics/semrel to SemRels/semrel

### Other Changes

* remove invalid secrets-in-if condition from semrel-release.yaml
* fix GHCR visibility — use PACKAGES_TOKEN PAT (GITHUB_TOKEN lacks write:packages scope)
* add one-shot workflow to set GHCR package visibility to public
* make ghcr.io/semrels/semrel container package public after push
* add JSON Schema badge, IDE setup section and schema link in config-reference

## v0.8.0 (2026-06-10)

### Features

* add JSON Schema for .semrel.yaml and wire into config init

### Other Changes

* **ci:** bump actions/checkout from 4 to 6

## v0.7.1 (2026-06-10)

### Bug Fixes

* resolve lint errors and test failures in cli package

### Other Changes

* **ci:** bump docker/build-push-action from 6 to 7
* fix stale go-semrel / GoSemantics references in README (#209)

## v0.7.0 (2026-06-10)

### Features

* **config:** add schemaVersion field and semrel migrate command (#195) (#208)
* **cli:** add semrel config command (#194) (#207)

## v0.6.0 (2026-06-10)

### Features

* **cli:** add --interactive flag to semrel release (#193) (#206)
* **cli:** add semrel changelog command (#192) (#205)

## v0.5.0 (2026-06-10)

### Features

* **cli:** add semrel doctor command (#191) (#204)

### Other Changes

* add E2E integration tests and full plugin smoke test suite (#203)

## v0.4.1 (2026-06-03)

### Bug Fixes

* **registry:** support @namespace/name refs in FindPlugin

## v0.4.0 (2026-06-03)

### Features

* pass SEMREL_COMMITS to plugins as JSON-encoded commit messages
* multi-arch plugin download via DownloadURLs map
* **release:** add pre-tag phase and version file commit via updater-go

### Bug Fixes

* **ci:** push pre-tag commit to main before tagging to prevent orphaned tags
* **ci:** fix SC2129 shellcheck warnings and add missing REUSE coverage
* **fmt:** gofmt internal/registry/metadata.go
* **ci:** sync working tree to released tag before GoReleaser
* **config:** add pre-tag to valid plugin phases

### Other Changes

* update status badge to v0.4.x
* **ci:** bump github/codeql-action (#198)
* **ci:** bump reviewdog/action-actionlint (#199)
* update ROADMAP milestone statuses and document SEMREL_COMMITS env var
* complete official plugin list in README
* update status from pre-alpha to alpha
* **config:** add TOML and JSON examples to config reference
* **changelog:** update for v0.3.3 [skip ci]

## v0.3.3 (2026-05-27)

### Bug Fixes

* **lint:** resolve golangci-lint v2 violations across codebase
* **lint:** fix errcheck violations flagged by golangci-lint v2

### Other Changes

* **changelog:** update for v0.3.1 [skip ci]

## v0.3.1 (2026-05-27)

### Bug Fixes

* **ci:** migrate golangci-lint config from v1 to v2 schema

### Other Changes

* **changelog:** update for v0.3.0 [skip ci]

## v0.3.0 (2026-05-27)

### Features

* **plugins:** add phase field (condition/release), run condition plugins before tagging, add condition-github-actions to self-release
* **release:** commit CHANGELOG.md to repo and handle tag_exists_strategy

### Bug Fixes

* **ci:** move CHANGELOG commit after GoReleaser to prevent tag/HEAD mismatch
* **ci:** push CHANGELOG commit to main before tag so GoReleaser finds correct HEAD
* **ci:** run test matrix on push to main, not only on PRs

### Other Changes

* **changelog:** update for v0.3.0 [skip ci]

## v0.3.0 (2026-05-27)

### Features

* **plugins:** add phase field (condition/release), run condition plugins before tagging, add condition-github-actions to self-release
* **release:** commit CHANGELOG.md to repo and handle tag_exists_strategy

### Bug Fixes

* **ci:** push CHANGELOG commit to main before tag so GoReleaser finds correct HEAD
* **ci:** run test matrix on push to main, not only on PRs

<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: 2026 The semrel Authors
-->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- GoReleaser appends release entries above this line -->
