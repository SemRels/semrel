# go-semrel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoSemantics/go-semrel)](https://goreportcard.com/report/github.com/GoSemantics/go-semrel)
[![CI](https://github.com/GoSemantics/go-semrel/actions/workflows/ci.yaml/badge.svg)](https://github.com/GoSemantics/go-semrel/actions/workflows/ci.yaml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/0/badge)](https://www.bestpractices.dev/projects/0)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/GoSemantics/go-semrel/badge)](https://scorecard.dev/viewer/?uri=github.com/GoSemantics/go-semrel)

> **Status: pre-alpha — actively under development. See [ROADMAP.md](ROADMAP.md).**
A Go-based semantic release system with a plugin architecture that automates the full release lifecycle — from commit analysis to publishing artefacts across multiple ecosystems.

## Features (planned)

- 🔍 **Conventional Commits** parser with configurable bump rules
- 📦 **Plugin architecture** — git, changelog, GitHub/GitLab Releases, npm, Docker, Helm, Go binaries, Python, Java, Rust, .NET, and more
- 📝 **Multi-format Changelog Engine** — CHANGELOG.md, ArtifactHub annotations, OCI labels, NuGet, PyPI, Slack/Teams/Discord excerpts
- 🏗️ **Monorepo support** — independent or synchronised versioning per package
- 🔐 **Supply-chain security** — signed releases, SBOM, SLSA provenance, Cosign
- ⚙️ **GitHub Actions** native integration

## Quick Start

```bash
# Install
go install github.com/GoSemantics/go-semrel/cmd/go-semrel@latest

# Dry-run release
go-semrel release --dry-run

# Lint commit messages
go-semrel lint
```

## Configuration

Copy `.semrel.yaml.example` to `.semrel.yaml` and adjust to your project. See [docs/config-reference.md](docs/config-reference.md) (coming soon).

## Documentation

- [Architecture Overview](docs/architecture.md) _(coming soon)_
- [Configuration Reference](docs/config-reference.md) _(coming soon)_
- [Plugin Development Guide](docs/plugin-development.md) _(coming soon)_
- [ROADMAP](ROADMAP.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions require a DCO sign-off (`git commit -s`).

## Security

Please report vulnerabilities via [GitHub Security Advisories](https://github.com/GoSemantics/go-semrel/security/advisories/new). See [SECURITY.md](SECURITY.md) for the full policy.

## License

Apache 2.0 — see [LICENSE](LICENSE).

Copyright 2026 The go-semrel Authors.
