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
