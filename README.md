# semrel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoSemantics/semrel)](https://goreportcard.com/report/github.com/GoSemantics/semrel)
[![CI](https://github.com/GoSemantics/semrel/actions/workflows/ci.yaml/badge.svg)](https://github.com/GoSemantics/semrel/actions/workflows/ci.yaml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/0/badge)](https://www.bestpractices.dev/projects/0)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/GoSemantics/semrel/badge)](https://scorecard.dev/viewer/?uri=github.com/GoSemantics/semrel)

> **Status: pre-alpha — actively under development. See [ROADMAP.md](ROADMAP.md).**

A Go-based semantic versioning and release system with a plugin architecture that automates the full release lifecycle. Designed for monorepos and multi-language projects.

## Features (planned)

- 🔍 **Conventional Commits** parser with configurable bump rules
- 📦 **Plugin architecture** — git, changelog, GitHub/GitLab Releases, npm, Docker, Helm, Go binaries, Python, Java, Rust, .NET, and more
- 📝 **Multi-format Changelog Engine** — CHANGELOG.md, ArtifactHub annotations, OCI labels, NuGet, PyPI, Slack/Teams/Discord excerpts
- 🏗️ **Monorepo support** — independent or synchronized versioning per package
- 🔐 **Supply-chain security** — signed releases, SBOM, SLSA provenance, Cosign
- ⚙️ **GitHub Actions** native integration
- 🔌 **gRPC Plugin Transport** — out-of-process plugins with hashicorp/go-plugin

## Quick Start

```bash
# Install
go install github.com/GoSemantics/semrel/cmd/go-semrel@latest

# Check version
semrel --version

# Validate commit messages
semrel lint

# Dry-run release (not yet implemented)
semrel release --dry-run
```

## Configuration

Copy `.semrel.yaml.example` to `.semrel.yaml` and adjust to your project. See [docs/config-reference.md](docs/config-reference.md) (coming soon).

## Architecture

- **Core Engine**: Conventional Commits analysis, SemVer calculation, changelog generation
- **Plugin System**: gRPC-based out-of-process plugins (see [ADR-001](docs/adr/ADR-001-grpc-plugin-transport.md))
- **Plugin SDK**: [`semrel-plugins`](https://github.com/GoSemantics/semrel-plugins) — reference SDK for external plugins

## Documentation

- [Architecture Overview](docs/adr/) — ADRs and design documents
- [Plugin Development Guide](https://github.com/GoSemantics/semrel-plugins) — Plugin SDK
- [Configuration Reference](docs/config-reference.md) _(coming soon)_
- [ROADMAP](ROADMAP.md) — public project roadmap

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions require:
- DCO sign-off (`git commit -s`)
- Conventional Commits
- REUSE/SPDX compliance

## Security

Please report vulnerabilities via [GitHub Security Advisories](https://github.com/GoSemantics/semrel/security/advisories/new). See [SECURITY.md](SECURITY.md) for the full policy.

## License

Apache 2.0 — see [LICENSE](LICENSE).

Copyright 2026 The semrel Authors.
