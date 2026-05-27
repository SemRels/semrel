# semrel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/GoSemantics/semrel)](https://goreportcard.com/report/github.com/GoSemantics/semrel)
[![CI](https://github.com/SemRels/semrel/actions/workflows/ci.yaml/badge.svg)](https://github.com/SemRels/semrel/actions/workflows/ci.yaml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/SemRels/semrel/badge)](https://scorecard.dev/viewer/?uri=github.com/SemRels/semrel)

> **Status: alpha (v0.3.x)** — core pipeline, plugin system, and CI/CD are fully functional. Self-versioned via semrel. Not yet recommended for production use; see [ROADMAP.md](ROADMAP.md) for the path to v1.0.0.

A Go-based semantic versioning and release system with a plugin architecture that automates the full release lifecycle. Designed for monorepos and multi-language projects.

## Features

- 🔍 **Conventional Commits** parser with configurable bump rules
- 🔌 **Rich ecosystem of standalone plugins** for providers, package updaters, and hooks
- 📝 **Multi-format changelog** — Markdown (Keep a Changelog), per-package monorepo changelogs
- 🏗️ **Monorepo support** — independent/lockstep versioning, package discovery, dependency graph, per-package changelogs
- 🔐 **Supply-chain security** — Cosign signing, CycloneDX/SPDX SBOM, SLSA Level 1 provenance
- ⚙️ **GitHub Actions** native integration
- 🧩 **Plugin runtime** — subprocess-based plugin execution from `~/.semrel/plugins/` or `$PATH`
- 🔗 **Issue tracking** — Jira and GitHub issue reference extraction from commit messages
- 📊 **Release analytics** — append-only NDJSON release history tracking
- ✅ **commitlint** — validate commit messages from CLI, git range, or stdin

## Installation

```bash
go install github.com/GoSemantics/semrel/cmd/semrel@latest
semrel --version
```

Install any plugins you want to use:

```bash
semrel plugin install github
semrel plugin install npm
```

## Quick Start

```bash
# Validate commit messages
semrel lint

# Dry-run release (preview what would happen)
semrel release --dry-run

# Run the full release pipeline
semrel release
```

## Available Plugins

| Plugin | Type | Repo |
|--------|------|------|
| github | Provider | [SemRels/provider-github](https://github.com/SemRels/provider-github) |
| gitlab | Provider | [SemRels/provider-gitlab](https://github.com/SemRels/provider-gitlab) |
| gitea | Provider | [SemRels/provider-gitea](https://github.com/SemRels/provider-gitea) |
| bitbucket | Provider | [SemRels/provider-bitbucket](https://github.com/SemRels/provider-bitbucket) |
| npm | Updater | [SemRels/updater-npm](https://github.com/SemRels/updater-npm) |
| docker | Updater | [SemRels/updater-docker](https://github.com/SemRels/updater-docker) |
| helm | Updater | [SemRels/updater-helm](https://github.com/SemRels/updater-helm) |
| cargo | Updater | [SemRels/updater-cargo](https://github.com/SemRels/updater-cargo) |
| python | Updater | [SemRels/updater-python](https://github.com/SemRels/updater-python) |
| gradle | Updater | [SemRels/updater-gradle](https://github.com/SemRels/updater-gradle) |
| maven | Updater | [SemRels/updater-maven](https://github.com/SemRels/updater-maven) |
| nuget | Updater | [SemRels/updater-nuget](https://github.com/SemRels/updater-nuget) |
| gobinary | Updater | [SemRels/updater-go](https://github.com/SemRels/updater-go) |
| homebrew | Updater | [SemRels/updater-homebrew](https://github.com/SemRels/updater-homebrew) |
| terraform | Updater | [SemRels/updater-terraform](https://github.com/SemRels/updater-terraform) |
| slack | Hook | [SemRels/hook-slack](https://github.com/SemRels/hook-slack) |
| matrix | Hook | [SemRels/hook-matrix](https://github.com/SemRels/hook-matrix) |
| email | Hook | [SemRels/hook-email](https://github.com/SemRels/hook-email) |
| jira | Hook | [SemRels/hook-jira](https://github.com/SemRels/hook-jira) |

## Configuration

Copy `.semrel.yaml.example` to `.semrel.yaml` and adjust it for your project. Plugin entries now refer to standalone binaries, for example:

```yaml
plugins:
  - uses: github
  - uses: npm
  - uses: docker
    args:
      image: myorg/myapp
```

See [docs/config-reference.md](docs/config-reference.md) for all options.

## Architecture

- **Core engine**: Conventional Commits analysis, SemVer calculation, changelog generation, git tag creation
- **Plugin system**: `pkg/plugininstance.Orchestrator` launches standalone plugin binaries in subprocesses
- **Plugin discovery**: `~/.semrel/plugins/semrel-plugin-<name>` first, then `$PATH`

See [docs/architecture.md](docs/architecture.md) for the full design.

## Documentation

- [Architecture Overview](docs/architecture.md) — pipeline design and component overview
- [Configuration Reference](docs/config-reference.md) — all `.semrel.yaml` options
- [Plugin Development Guide](docs/plugin-development.md) — build standalone plugins
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