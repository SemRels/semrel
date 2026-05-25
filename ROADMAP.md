# Roadmap

This document describes the planned milestones for semrel. For detailed issue tracking see the [GitHub Milestones](https://github.com/SemRels/semrel/milestones).

## What we are building

A Go-native, plugin-based semantic release system that covers the full release lifecycle for any language ecosystem — inspired by semantic-release (Node.js) but designed from the ground up for Go and cloud-native environments.

## What we are NOT doing (scope boundaries)

- We are not a build tool (no compilation, packaging beyond what plugins handle)
- We are not a monorepo management tool (we integrate with them, not replace them)
- We are not opinionated about branching strategies beyond what is configured

---

## v0.0.1 — Project Foundation _(current)_
**Goal:** Establish governance, legal and supply-chain security foundations.

- Apache 2.0 license, SPDX headers, REUSE compliance
- GOVERNANCE.md, MAINTAINERS.md, CODE_OF_CONDUCT.md, SECURITY.md, ADOPTERS.md
- DCO enforcement, branch protection, hardened CI workflows
- OpenSSF Best Practices badge, OpenSSF Scorecard, Renovate
- ADR directory (`docs/adr/`)

## v0.1.0 — Foundation
**Goal:** Working CLI with core release engine and basic git integration.

- Go module scaffold, Makefile, golangci-lint
- SemVer parser and version bumping logic (incl. vector model from concept doc)
- Conventional Commits parser (feat/fix/breaking, configurable type→bump mapping)
- Release rules configuration (`.semrel.yaml` — branches, tag prefix, bump rules)
- Maintenance branch backport support (`1.x`, `2.x` patterns)
- Git tag management (read, create annotated tag, push)
- `semrel release --dry-run`, `--force-bump-patch-version`, `--edit` flags
- ADR docs: `docs/adr/`

## v0.1.5 — Plugin Transport (gRPC)
**Goal:** Out-of-process plugin foundation using hashicorp/go-plugin + gRPC.

- Protocol Buffers v3: `plugins/v1/proto/semantic_release.proto`
  - RPCs: `VerifyConditions`, `AnalyzeCommits`, `GenerateNotes`, `Prepare`, `Publish`, `OnSuccess`, `OnFail`
- hashicorp/go-plugin host integration
- Plugin discovery: `.semrel/<GOOS>_<GOARCH>/<name>/<version>/`
- Plugin registry client + auto-download
- Air-gapped support (pre-populated `.semrel/` dir)
- Logging contract: stdout reserved for handshake, all logs to stderr
- Built-in plugin categories: **CI Condition**, **Provider**, **Commit Analyzer**, **Changelog Generator**, **Files Updater**, **Hooks**

## v0.2.0 — Plugin System ✅ (shipped)
**Goal:** First-party plugins covering the core release workflow.

- Plugin `Executor` interface and in-process `Registry` (`pkg/plugin`)
- `pkg/builtins.DefaultRegistry()` wired into release pipeline (step 12)
- Built-in plugins: `github`, `gitlab`, `gitea`, `npm`, `docker`, `helm`, `cargo`, `python`, `gradle`, `maven`, `gobinary`, `slack`, `matrix`

## v0.2.5 — Changelog Engine
**Goal:** Multi-format changelog rendering from a single source of truth.

- Structured `ReleaseNotes` model and `Renderer` / `FileUpdater` interfaces
- Formats: Keep-a-Changelog (CHANGELOG.md), GitHub/GitLab Release Notes, Helm ArtifactHub annotations, OCI image labels, NuGet `<ReleaseNotes>`, PyPI CHANGES.rst, notification excerpts (Slack/Teams/Discord/Matrix), RSS/Atom
- Custom Go `text/template` files for any format
- Per-package changelogs in monorepos

## v0.3.0 — CI Integration ✅ (shipped)
**Goal:** First-class GitHub Actions support and structured output.

- `action.yml` composite action
- Structured JSON release output (`--output-format json`)
- End-to-end integration tests
- Reusable workflow templates (pending: needs `workflow` OAuth scope — issues #17, #76)

## v0.4.0 — Ecosystem Plugins ✅ (shipped)
**Goal:** Support every major language ecosystem and forge.

- Forges: GitLab Releases, Gitea/Forgejo ✅
- Languages: Go binaries (cross-compile), Python/PyPI, Java Gradle, Java Maven, Rust/Cargo, Helm ✅
- Infrastructure: Homebrew Tap, Terraform/OpenTofu registry, OCI/ORAS artifacts (packages exist, not yet in DefaultRegistry)
- Security: SBOM (CycloneDX/SPDX), Cosign signing, SLSA provenance
- Notifications: Discord, Microsoft Teams, Generic HTTP webhook

## v0.5.0 — Monorepo & Advanced
**Goal:** Full monorepo support and advanced release workflows.

- Workspace/package discovery (auto-detect Go, npm, Python, Java, Helm, ...)
- Independent versioning per package with tag namespacing (`packages/api/v1.2.3`)
- Synchronised / lockstep versioning mode
- Inter-package dependency graph and topological release ordering
- Plugin instancing (multiple instances of same plugin with different configs)
- Pre-release channels (alpha/beta/rc) per branch
- Release lock, Commitlint, Interactive release notes editor
- Jira/Linear issue auto-close, Release analytics, Plugin SDK for third-party plugins

