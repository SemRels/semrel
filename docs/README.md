# semrel Documentation

This directory contains architecture documentation, design decisions, and technical guides for the semrel project.

## Contents

### Architecture Decision Records (ADRs)

ADRs document significant technical decisions and their rationale:

See [docs/adr/README.md](./adr/README.md) for the current ADR index.

### Godoc Overview

The [docs/doc.go](./doc.go) file provides package-level documentation for this directory.

To read the overview as formatted HTML:
```bash
go doc -html github.com/SemRels/semrel/docs | open /dev/stdin
```

## Release Pipeline

semrel follows a 12-step release pipeline:

1. **Load Configuration** — Parse `.semrel.yaml` (or `.toml`/`.json`)
2. **Check Updates** — Advisory check for newer CLI and configured plugin versions
3. **Check Conditions** — Run gate plugins (branches, status checks, etc.)
4. **Gather Commits** — Collect commits since last release tag
5. **Parse Commits** — Parse Conventional Commits (feat, fix, breaking changes)
6. **Calculate Version** — Determine next SemVer using bump rules
7. **Generate Release Notes** — Build changelog from commits
8. **Run Pre-Release Plugins** — Execute updaters (version files, etc.)
9. **Commit Release Changes** — Commit all tracked changelog and pre-release plugin changes once
10. **Tag Release** — Create annotated Git tag and push
11. **Run Post-Release Plugins** — Execute providers (GitHub, GitLab, etc.) and hooks (Slack, Teams, etc.)
12. **Report Results** — Output release summary

## Plugin Architecture

semrel uses a modular plugin system with three categories:

### Providers
Publish releases to forges and registries:
- **provider-github** — GitHub Releases API
- **provider-gitlab** — GitLab Releases API
- **provider-gitea** — Gitea API
- **provider-bitbucket** — Bitbucket Cloud API

### Updaters
Update version files and dependency manifests:
- **updater-npm** — package.json version
- **updater-cargo** — Cargo.toml version
- **updater-python** — setup.py, pyproject.toml versions
- **updater-docker** — Dockerfile tag updates
- **updater-helm** — Chart.yaml version
- **updater-maven** — pom.xml version
- **updater-gradle** — build.gradle version
- **updater-go** — Internal version constants
- **updater-nuget** — .csproj version
- **updater-terraform** — version.tf updates
- **updater-homebrew** — Homebrew formula updates

### Hooks
Notify teams and log releases:
- **hook-slack** — Send Slack notifications
- **hook-teams** — Send Microsoft Teams messages
- **hook-email** — Send email notifications
- **hook-jira** — Update JIRA issues
- **hook-matrix** — Send Matrix/Element messages
- **hook-gitplugin** — Custom Git hooks

## Configuration Reference

### Minimal .semrel.yaml

```yaml
schemaVersion: 1

branches:
  - name: main
```

### Full Configuration Example

```yaml
schemaVersion: 1

branches:
  - name: main
    prerelease: ""
  - name: next
    prerelease: beta
  - name: develop
    prerelease: alpha

rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: perf
    bump: minor
  - type: breaking
    bump: major

plugins:
  - uses: github
    name: github_provider
    with:
      token: ${GITHUB_TOKEN}

  - uses: npm
    name: npm_public
    with:
      registry: https://registry.npmjs.org

  - uses: npm
    name: npm_private
    with:
      registry: https://internal.company.com

  - uses: slack
    name: slack_notify
    with:
      webhook: ${SLACK_WEBHOOK}
      channel: "#releases"

  - uses: teams
    name: teams_notify
    with:
      webhook: ${TEAMS_WEBHOOK}

workspace:
  strategy: independent
  packages:
    - name: core
      path: packages/core
    - name: api
      path: packages/api
      dependsOn: [core]
    - name: web
      path: packages/web
      dependsOn: [api]
  lock:
    enabled: false
    redis: ""
```

### Configuration Fields

#### schemaVersion
Version of the configuration schema. Currently: `1`

#### branches
List of release branches:
- **name** (required): Branch name (e.g., "main", "next")
- **prerelease** (optional): Pre-release channel (e.g., "alpha", "beta", "rc")
- **maintenance** (optional): True if this is a maintenance branch (e.g., "1.x")

#### rules
Commit type to version bump mapping:
- **type**: Conventional commit type or "breaking"
- **bump**: "major", "minor", or "patch"

#### plugins
External plugin configurations:
- **uses**: Plugin name (without "semrel-plugin-" prefix)
- **name**: Instance name (allows multiple instances of same plugin)
- **with**: Plugin configuration (key-value pairs, supports ${VAR} substitution)

#### workspace
Monorepo configuration:
- **strategy**: "independent" (each package has own version) or "lockstep" (shared version)
- **packages**: List of packages with paths and dependencies
- **lock**: Distributed lock configuration (Redis)

#### Other Options
- **tagPrefix**: Prefix for release tags (default: "v")
- **versionCeiling**: Maximum version to release (e.g., "1.x" or "2.0.0")
- **commitChangelog**: Whether to commit CHANGELOG.md (default: true)
- **tagExistsStrategy**: How to handle existing tags ("fail", "overwrite", etc.)

## Using semrel

### Release a Single Package

```bash
semrel release
```

With options:
```bash
semrel release --dry-run              # Simulate without pushing
semrel release --edit                 # Edit changelog before committing
semrel release --force-bump-patch     # Force patch version bump
```

### Release a Monorepo

```bash
semrel workspace release

# With options:
semrel workspace release --parallel   # Release independent packages in parallel
semrel workspace release --fail-fast  # Stop on first failure
```

### Generate Changelog Only

```bash
semrel changelog
semrel changelog --from v1.0.0 --to v2.0.0
```

### Manage Plugins

```bash
semrel plugin install slack
semrel plugin list
semrel plugin update --check
semrel plugin restore
```

## Testing semrel

### Run All Tests

```bash
go test ./...
```

### Run Package-Specific Tests

```bash
go test ./pkg/releasenotes -v
go test ./pkg/monorepo -v
go test ./pkg/commits -v
go test ./pkg/semver -v
go test ./internal/cli -v
```

### Run Tests with Coverage

```bash
go test ./... -cover
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests

Integration tests use a temporary Git repository:

```bash
go test ./internal/cli -run TestReleaseCommandIntegration -v
go test ./internal/cli -run TestWorkspaceReleaseIntegration -v
```

## Contributing

When adding new features:

1. **Write tests first** — Follow TDD
2. **Document in ADR** — If it's a significant design decision
3. **Update configuration schema** — If it affects `.semrel.yaml`
4. **Add to godoc** — Document public functions and types
5. **Update README.md** — Add usage examples if needed

### Creating a Plugin

Plugins are standalone Go binaries. Use the [plugin-template](../plugin-template/) as a starting point:

```bash
git clone https://github.com/SemRels/plugin-template.git semrel-plugin-myname
cd semrel-plugin-myname
```

Implement the `sdk.Plugin` interface and distribute as `~/.semrel/plugins/semrel-plugin-myname`.

### Creating a Changelog Renderer

Add a new `Render*()` method to `pkg/releasenotes/ReleaseNotes`:

```go
func (r *ReleaseNotes) RenderMyFormat() string {
	// Your implementation
}
```

Write tests in `pkg/releasenotes/*_test.go`.

## License

All documentation and code in this project is licensed under the Apache 2.0 License.
SPDX: Apache-2.0

See [LICENSE](../LICENSE) for details.
