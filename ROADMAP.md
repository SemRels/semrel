# Roadmap

This document describes the planned milestones for go-semrel. For detailed issue tracking see the [GitHub Milestones](https://github.com/GoSemantics/go-semrel/milestones).

## What we are building

A Go-native, plugin-based semantic release system that covers the full release lifecycle for any language ecosystem — inspired by semantic-release (Node.js) but designed from the ground up for Go and cloud-native environments.

## What we are NOT doing (scope boundaries)

- We are not a build tool (no compilation, packaging beyond what plugins handle)
- We are not a monorepo management tool (we integrate with them, not replace them)
- We are not opinionated about branching strategies beyond what is configured

---

## v0.0.1 — CNCF Foundation _(current)_
**Goal:** Establish all governance, legal and supply-chain security foundations required for CNCF Sandbox eligibility.

- Apache 2.0 license, SPDX headers, REUSE compliance
- GOVERNANCE.md, MAINTAINERS.md, CODE_OF_CONDUCT.md (CNCF), SECURITY.md
- DCO enforcement, branch protection, hardened CI workflows
- OpenSSF Best Practices badge, OpenSSF Scorecard, Dependabot
- CLOMonitor registration

## v0.1.0 — Foundation
**Goal:** Working CLI with core release engine and basic git integration.

- Go module scaffold, Makefile, golangci-lint
- SemVer parser and version bumping logic
- Conventional Commits parser
- Release rules configuration (`.semrel.yaml`)
- Git tag management (read, create, push)
- `--dry-run` mode

## v0.2.0 — Plugin System
**Goal:** Extensible plugin architecture with first-party plugins.

- Plugin interface (lifecycle hooks: Init, Prepare, Publish, Success, Fail)
- Plugin loader and registry
- Built-in plugins: `git`, `changelog`, `github-releases`, `npm`, `docker`

## v0.2.5 — Changelog Engine
**Goal:** Multi-format changelog rendering from a single source of truth.

- Structured `ReleaseNotes` model and `Renderer` interface
- Formats: Keep-a-Changelog, GitHub/GitLab Releases, Helm ArtifactHub, OCI annotations, NuGet, PyPI, notification excerpts, custom Go templates, RSS/Atom

## v0.3.0 — CI Integration
**Goal:** First-class GitHub Actions support and notification plugins.

- `action.yml` and reusable workflow templates
- Notification plugins: Gitter/Matrix, Slack
- End-to-end integration tests
- Structured JSON release output for downstream steps

## v0.4.0 — Ecosystem Plugins
**Goal:** Support every major language ecosystem and forge.

- Plugins: GitLab Releases, Helm, Go binaries, Python/PyPI, Java Gradle, Java Maven, Rust/Cargo, .NET/NuGet, Homebrew Tap, Terraform, OCI/ORAS, SBOM, Cosign/SLSA, Gitea/Forgejo
- Notifications: Discord, Microsoft Teams, Generic Webhook

## v0.5.0 — Monorepo & Advanced
**Goal:** Full monorepo support and advanced release workflows.

- Workspace/package discovery
- Independent and synchronised versioning per package
- Plugin instancing (multiple instances of same plugin with different configs)
- Pre-release channels (alpha/beta/rc) per branch
- Release lock, Commitlint integration, Interactive release notes editor
- Jira/Linear issue auto-close, Plugin SDK for third-party plugins
