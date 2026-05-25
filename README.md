# semrel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoSemantics/semrel)](https://goreportcard.com/report/github.com/GoSemantics/semrel)
[![CI](https://github.com/SemRels/semrel/actions/workflows/ci.yaml/badge.svg)](https://github.com/SemRels/semrel/actions/workflows/ci.yaml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/0/badge)](https://www.bestpractices.dev/projects/0)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/SemRels/semrel/badge)](https://scorecard.dev/viewer/?uri=github.com/SemRels/semrel)

> **Status: pre-alpha — actively under development. See [ROADMAP.md](ROADMAP.md).**

A Go-based semantic versioning and release system with a plugin architecture that automates the full release lifecycle. Designed for monorepos and multi-language projects.

## Features

- 🔍 **Conventional Commits** parser with configurable bump rules
- 📦 **13 built-in plugins** — GitHub Releases, GitLab, Gitea, npm, Docker, Helm, Cargo, PyPI, Gradle, Maven, Go binary, Slack, Matrix
- 📝 **Multi-format Changelog** — Markdown (Keep a Changelog), per-package monorepo changelogs
- 🏗️ **Monorepo support** — independent/lockstep versioning, package discovery, dependency graph, per-package changelogs
- 🔐 **Supply-chain security** — Cosign signing, CycloneDX/SPDX SBOM, SLSA Level 1 provenance
- ⚙️ **GitHub Actions** native integration
- 🔌 **Plugin SDK** — build in-process plugins with the official Go interface; external gRPC plugins planned
- 🔗 **Issue tracking** — Jira and GitHub issue reference extraction from commit messages
- 📊 **Release analytics** — append-only NDJSON release history tracking
- ✅ **commitlint** — validate commit messages from CLI, git range, or stdin

## Quick Start

```bash
# Install
go install github.com/GoSemantics/semrel/cmd/semrel@latest

# Check version
semrel --version

# Validate commit messages
semrel lint

# Dry-run release (preview what would happen)
semrel release --dry-run

# Run the full release pipeline
semrel release
```

## Configuration

Copy `.semrel.yaml.example` to `.semrel.yaml` and adjust to your project. See [docs/config-reference.md](docs/config-reference.md) for all options.

## Architecture

- **Core Engine**: Conventional Commits analysis, SemVer calculation, changelog generation, git tag creation
- **Plugin System**: 13 in-process built-in plugins (`pkg/builtins`); external gRPC plugins planned (see [docs/architecture.md](docs/architecture.md))
- **Plugin Registry**: `https://semrels.github.io/semrel-registry` — community plugin discovery (in development)

## Documentation

- [Architecture Overview](docs/architecture.md) — pipeline design and component overview
- [Configuration Reference](docs/config-reference.md) — all `.semrel.yaml` options
- [Plugin Development Guide](docs/plugin-development.md) — build custom plugins
- [CNCF Due Diligence](docs/cncf-due-diligence.md) — project overview for CNCF Sandbox application
- [ADRs](docs/adr/) — architectural decision records
- [ROADMAP](ROADMAP.md) — public project roadmap

## Supply Chain Security

semrel takes supply-chain security seriously:

- **Signed releases**: Artifacts signed with [Sigstore Cosign](https://github.com/sigstore/cosign) (keyless OIDC)
- **SBOM**: CycloneDX 1.4 and SPDX 2.3 Bills of Materials published per release
- **SLSA provenance**: Level 1 build provenance documenting artifact digests
- **DCO**: Developer Certificate of Origin required on all commits
- **REUSE/SPDX**: License compliance enforced in CI on every PR

See [SECURITY.md](SECURITY.md) for vulnerability reporting and artifact verification instructions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions require:
- DCO sign-off (`git commit -s`)
- Conventional Commits
- REUSE/SPDX compliance

## Security

Please report vulnerabilities via [GitHub Security Advisories](https://github.com/SemRels/semrel/security/advisories/new). See [SECURITY.md](SECURITY.md) for the full policy.

## License

Apache 2.0 — see [LICENSE](LICENSE).

Copyright 2026 The semrel Authors.
