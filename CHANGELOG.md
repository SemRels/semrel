## v0.13.0 (2026-06-11)

### Features

* **schema:** add workspace config to JSON Schema v1
* **workspace:** dry-run everywhere + skip packages with nothing to release
* **workspace:** add semrel workspace command for monorepo orchestration

### Bug Fixes

* **doctor:** search for ecosystem files recursively up to 4 levels deep
* **lint:** address all golangci-lint issues

### Other Changes

* add workspace config section to config-reference.md

## v0.12.6 (2026-06-11)

### Bug Fixes

* **plugin:** restore verifies checksum before skipping existing binary

## v0.12.5 (2026-06-11)

### Bug Fixes

* **registry,doctor:** FindVersion returns oldest instead of newest + doctor false positives

## v0.12.4 (2026-06-11)

### Bug Fixes

* **lock:** rename release mutex to .semrel-release.lock; skip release plugins in dry-run
* **plugin:** empty args must not shadow SEMREL_PLUGIN_* env vars
* **release:** dry-run plugin failures are warnings, not hard errors

## v0.12.3 (2026-06-11)

### Bug Fixes

* **git:** resolve detached-HEAD branch via CI environment variables

## v0.12.2 (2026-06-11)

### Bug Fixes

* **doctor:** suppress suggestions for already-configured @namespace/ plugins
* **docker:** pin Alpine, add apk upgrade, fix distroless missing git

### Other Changes

* fix .semrel/plugins/ path references (project-local takes priority)
* add InterFace AG as first production adopter

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

## v0.26.3 (2026-09-21)

### Bug Fixes

* **ci:** bump codeql-action init+analyze together to v4.38.1

### Other Changes

* **ci:** bump github/codeql-action/upload-sarif from 4.37.9 to 4.38.1 (#294)
* **ci:** bump docker/setup-buildx-action from 4.3.0 to 4.4.1 (#295)
* **ci:** bump reviewdog/action-actionlint from 1.73.4 to 1.76.0 (#296)
* **ci:** bump docker/setup-qemu-action from 4.2.0 to 4.3.0 (#286)
* **ci:** bump docker/setup-buildx-action from 4.2.0 to 4.3.0 (#287)
* **ci:** bump reviewdog/action-actionlint from 1.73.2 to 1.73.4 (#288)
* **ci:** bump renovatebot/github-action from 46.2.2 to 46.2.4 (#284)
* **ci:** bump github/codeql-action/upload-sarif from 4.37.8 to 4.37.9 (#282)
* **ci:** bump actions/setup-go from 6 to 7 (#281)

## v0.26.2 (2026-08-30)

### Bug Fixes

* **config:** preserve explicit empty tag prefix (#268)

## v0.26.1 (2026-08-25)

### Bug Fixes

* keep immutable Docker tag conflicts non-blocking

## v0.26.0 (2026-08-25)

### Features

* **config:** support env var interpolation in config files
* **cli:** add --commit-msg-file flag to commitlint (#264)

### Bug Fixes

* resolve release metadata without local tags
* preserve release metadata when Docker publication fails (#278)
* let releases proceed past Docker tag conflicts (#277)
* tolerate idempotent image manifest retries (#269)
* **ci:** publish release changelog through pull request (#266)
* **config:** honor explicit empty tagPrefix

### Other Changes

* **ci:** bump docker/metadata-action from 5.9.0 to 6.2.0 (#275)
* **ci:** bump reviewdog/action-actionlint from 1.73.1 to 1.73.2 (#274)
* **ci:** bump github/codeql-action/upload-sarif from 4.36.3 to 4.37.8 (#273)
* **ci:** bump renovatebot/github-action from 46.1.18 to 46.2.2 (#272)
* **deps:** bump github.com/stretchr/testify from 1.12.0 to 1.12.1 (#270)
* **ci:** bump ossf/scorecard-action from 2.4.3 to 2.4.4 (#271)
* **ci:** remove GoReleaser release pipeline (#267)

## v0.25.4 (2026-08-17)

### Bug Fixes

* migrate cosign signing to v3 `--bundle` format

## v0.25.3 (2026-08-17)

### Bug Fixes

* restore original release asset naming and cosign signing
* remove `commit_changelog: false` when no replacement changelog plugin is configured

## v0.25.2 (2026-08-17)

### Bug Fixes

* move `-trimpath` from linker flags to `go build` flags

## v0.25.1 (2026-08-17)

### Bug Fixes

* add the correct `cmd/semrel` entry point to GoReleaser

## v0.25.0 (2026-08-17)

### Bug Fixes

* preserve release outputs when tag publication warns
* keep the release pipeline running on tag publication errors
* create tags through the GitHub REST API fallback
* authenticate and verify GitHub tag publication

### Other Changes

* **ci:** bump docker/login-action from 4.5.0 to 4.6.0 (#245)
* **ci:** bump goreleaser/goreleaser-action from 7.2.2 to 7.2.3 (#246)
* **ci:** bump actions/checkout from 6.0.3 to 7.0.1 (#247)

## v0.24.0 (2026-07-06)

### Features

* **core:** pass contributor metadata to plugins

## v0.23.5 (2026-07-01)

### Bug Fixes

* **release:** use GOPROXY=direct when installing provider-github plugin (#233)

## v0.23.4 (2026-07-01)

### Bug Fixes

* **release:** reference correct step output for version in asset build/publish (#232)

## v0.23.3 (2026-07-01)

### Bug Fixes

* **release:** disable cosign use-signing-config to allow legacy sig/cert output (#231)

## v0.23.2 (2026-07-01)

### Bug Fixes

* **release:** disable the cosign new bundle format for signature and certificate output (#230)

## v0.23.1 (2026-07-01)

### Bug Fixes

* remove the leftover GoReleaser env block that caused duplicate `env` keys (#229)

### Other Changes

* replace GoReleaser with semrel's provider-github plugin (#227)
* add Go 1.27rc1 to the test matrix (#226)
* use a dynamic release badge instead of a hardcoded version (#225)

## v0.23.0 (2026-07-01)

### Features

* **release:** sign release checksums with keyless cosign (#223)

### Other Changes

* bump sigstore/cosign-installer from 3.9.1 to 4.1.2
* bump actions/setup-go from 5 to 6
* update golangci/golangci-lint-action (#213)
* bump docker/setup-buildx-action from 3.12.0 to 4.1.0 (#215)

## v0.22.1 (2026-07-01)

### Bug Fixes

* format config test code (#221)

## v0.22.0 (2026-06-30)

### Features

* **config:** improve GitHub Actions defaults and user prompts

### Bug Fixes

* reduce the download-tracking timeout for a better offline experience

## v0.21.0 (2026-06-30)

### Bug Fixes

* enforce `go.mod` and `go.sum` consistency
* improve plugin download tracking synchronization

## v0.20.1 (2026-06-30)

### Bug Fixes

* improve download tracking for plugin installs and restores

## v0.20.0 (2026-06-30)

### Features

* **download:** track plugin downloads

## v0.19.0 (2026-06-30)

### Features

* support scope-based versioning in release rules and improve validation

## v0.18.0 (2026-06-28)

### Features

* add update and restore commands for plugin management
* add Packager and Publisher plugin categories

### Bug Fixes

* resolve REUSE 3.3 compliance issues

## v0.17.1 (2026-06-24)

### Bug Fixes

* resolve REUSE 3.3 compliance issues

## v0.17.0 (2026-06-15)

### Features

* **plugin:** add update and restore commands for plugin management

## v0.16.0 (2026-06-12)

### Features

* track release analytics
* preserve plugin category prefixes in plugin binary names

### Other Changes

* add the Plugin Development Guide
* bump actions/setup-go from v5 to v6

## v0.15.0 (2026-06-12)

### Features

* add pre-release channels, workspace lockstep, and `--next-version` override

### Bug Fixes

* remove the gobinary pre-tag plugin to avoid pushing commits to protected `main`
* fix lint and scorecard workflow issues

### Other Changes

* pin scorecard action SHAs and add fuzzing

## v0.14.0 (2026-06-11)

### Features

* add the generator phase, allowing plugin output to override `SEMREL_CHANGELOG`

### Bug Fixes

* clarify that `commit_changelog: false` disables the built-in changelog write
