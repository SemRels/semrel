# semrel

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/SemRels/semrel)](https://goreportcard.com/report/github.com/SemRels/semrel)
[![CI](https://github.com/SemRels/semrel/actions/workflows/ci.yaml/badge.svg)](https://github.com/SemRels/semrel/actions/workflows/ci.yaml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/SemRels/semrel/badge)](https://scorecard.dev/viewer/?uri=github.com/SemRels/semrel)
[![JSON Schema](https://img.shields.io/badge/schema-registry.semrel.io%2Fschemas%2Fcore%2Fv1.json-blue)](https://registry.semrel.io/schemas/core/v1.json)

> **Status: v0.15.0** — core pipeline, plugin system, and CI/CD are fully functional. Self-versioned via semrel. Not yet recommended for production use; see [ROADMAP.md](ROADMAP.md) for the path to v1.0.0.

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
go install github.com/SemRels/semrel/cmd/semrel@latest
semrel --version
```

```bash
# Update to the latest release
semrel update
```

Install any plugins you want to use:

```bash
semrel plugin install github
semrel plugin install npm
```

## Quick Start

```bash
# Create .semrel.yaml
semrel config init

# Verify config, plugins, git state, and environment
semrel doctor

# Validate commit messages since the last tag
semrel lint

# Dry-run release (preview what would happen)
semrel release --dry-run

# Run the full release pipeline
semrel release

# Update semrel itself
semrel update
```

## Commands

| Command | Description |
|---------|-------------|
| `semrel release` | Run the full release pipeline from commit analysis through tagging and release plugins. |
| `semrel changelog` | Generate unreleased changelog entries without creating a release. |
| `semrel lint` | Validate commits since the last tag against Conventional Commits. |
| `semrel commitlint` | Lint commit messages from the last tag, a git range, arguments, or stdin. |
| `semrel doctor` | Run pre-flight checks for config, plugins, git state, and environment. |
| `semrel config init` | Create a new `.semrel.yaml` configuration file. |
| `semrel config show` | Print the resolved configuration. |
| `semrel config validate` | Validate the current configuration file. |
| `semrel config set` | Update a top-level or nested configuration key. |
| `semrel migrate` | Upgrade `.semrel.yaml` to the current schema version. |
| `semrel plugin list` | List plugins available in the registry. |
| `semrel plugin search` | Search plugins by name, description, or tag. |
| `semrel plugin install` | Download and install a plugin binary. |
| `semrel update` | Check for and install the latest semrel release. |

## IDE Support

The `.semrel.yaml` config file has a published [JSON Schema](https://registry.semrel.io/schemas/core/v1.json) that enables auto-complete, validation, and hover docs in VS Code, JetBrains IDEs, and any LSP-aware editor.

`semrel config init` automatically adds the schema directive. For existing configs, add this line at the top:

```yaml
# yaml-language-server: $schema=https://registry.semrel.io/schemas/core/v1.json
```

VS Code users: install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) for full support.

## Local Production Demo (semrel + real plugins)

Run an end-to-end local demo that builds `semrel`, builds real plugin binaries from sibling repositories, creates a temporary git repository, and runs a release pipeline using external plugins:

```bash
make local-demo
```

Optional dry-run mode:

```bash
bash scripts/local-demo.sh --dry-run
```

Expected outcome:
- a new demo repository is created in your workspace root
- semrel executes external plugins via `path:` entries
- a new semantic version tag is created (unless `--dry-run` is used)

## Available Plugins

Install any plugin with `semrel plugin install <name>`.

| Plugin | Type | Description | Repo |
|--------|------|-------------|------|
| **Conditions** | | | |
| `github-actions` | Condition | Allow releases only on GitHub Actions CI | [SemRels/condition-github-actions](https://github.com/SemRels/condition-github-actions) |
| `gitlab-ci` | Condition | Allow releases only on GitLab CI | [SemRels/condition-gitlab-ci](https://github.com/SemRels/condition-gitlab-ci) |
| `gitea-actions` | Condition | Allow releases only on Gitea Actions | [SemRels/condition-gitea-actions](https://github.com/SemRels/condition-gitea-actions) |
| `generic` | Condition | Generic CI environment condition | [SemRels/condition-generic](https://github.com/SemRels/condition-generic) |
| **Analyzers** | | | |
| `conventional` | Analyzer | Conventional Commits commit analyzer | [SemRels/analyzer-conventional](https://github.com/SemRels/analyzer-conventional) |
| `default` | Analyzer | Default commit analyzer | [SemRels/analyzer-default](https://github.com/SemRels/analyzer-default) |
| **Generators** | | | |
| `changelog-md` | Generator | Markdown changelog (Keep a Changelog format) | [SemRels/generator-changelog-md](https://github.com/SemRels/generator-changelog-md) |
| `changelog-html` | Generator | HTML changelog generator | [SemRels/generator-changelog-html](https://github.com/SemRels/generator-changelog-html) |
| `release-notes` | Generator | Release notes generator | [SemRels/generator-release-notes](https://github.com/SemRels/generator-release-notes) |
| **Providers** | | | |
| `github` | Provider | GitHub releases, assets, and tags | [SemRels/provider-github](https://github.com/SemRels/provider-github) |
| `gitlab` | Provider | GitLab releases and tags | [SemRels/provider-gitlab](https://github.com/SemRels/provider-gitlab) |
| `gitea` | Provider | Gitea releases and tags | [SemRels/provider-gitea](https://github.com/SemRels/provider-gitea) |
| `bitbucket` | Provider | Bitbucket releases and tags | [SemRels/provider-bitbucket](https://github.com/SemRels/provider-bitbucket) |
| `git` | Provider | Local git tag only (no platform integration) | [SemRels/provider-git](https://github.com/SemRels/provider-git) |
| **Updaters** | | | |
| `npm` | Updater | Bump `package.json` version | [SemRels/updater-npm](https://github.com/SemRels/updater-npm) |
| `docker` | Updater | Build and push Docker images | [SemRels/updater-docker](https://github.com/SemRels/updater-docker) |
| `helm` | Updater | Bump Helm chart version | [SemRels/updater-helm](https://github.com/SemRels/updater-helm) |
| `cargo` | Updater | Publish Rust/Cargo crate | [SemRels/updater-cargo](https://github.com/SemRels/updater-cargo) |
| `python` | Updater | Bump PyPI package version | [SemRels/updater-python](https://github.com/SemRels/updater-python) |
| `gradle` | Updater | Bump Gradle version | [SemRels/updater-gradle](https://github.com/SemRels/updater-gradle) |
| `maven` | Updater | Publish Maven artifact | [SemRels/updater-maven](https://github.com/SemRels/updater-maven) |
| `nuget` | Updater | Bump NuGet package version | [SemRels/updater-nuget](https://github.com/SemRels/updater-nuget) |
| `gobinary` | Updater | Update Go version variable in source | [SemRels/updater-go](https://github.com/SemRels/updater-go) |
| `homebrew` | Updater | Update Homebrew formula | [SemRels/updater-homebrew](https://github.com/SemRels/updater-homebrew) |
| `terraform` | Updater | Bump Terraform module version | [SemRels/updater-terraform](https://github.com/SemRels/updater-terraform) |
| **Hooks** | | | |
| `slack` | Hook | Send release notifications to Slack | [SemRels/hook-slack](https://github.com/SemRels/hook-slack) |
| `teams` | Hook | Send release notifications to Microsoft Teams | [SemRels/hook-teams](https://github.com/SemRels/hook-teams) |
| `matrix` | Hook | Send release notifications to Matrix/Element | [SemRels/hook-matrix](https://github.com/SemRels/hook-matrix) |
| `email` | Hook | Send release notification emails | [SemRels/hook-email](https://github.com/SemRels/hook-email) |
| `jira` | Hook | Transition Jira issues on release | [SemRels/hook-jira](https://github.com/SemRels/hook-jira) |
| `gitplugin` | Hook | Run arbitrary git operations post-release | [SemRels/hook-gitplugin](https://github.com/SemRels/hook-gitplugin) |

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