<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: 2026 The semrel Authors
-->

# MVP Roadmap: semrel v0.1.0

This document tracks the minimal viable product (MVP) for `semrel v0.1.0`. These features enable:
- ✅ `semrel lint` — validate commit messages
- ✅ `semrel release --dry-run` — analyze next version and show changelog

## Phase 1: Core Engine (HIGHEST PRIORITY)

### 1. Conventional Commits Parser
**Issue:** [#3 Conventional Commits parser](https://github.com/GoSemantics/semrel/issues/3)

Parse commit messages following the [Conventional Commits](https://www.conventionalcommits.org/) specification.

**Implementation:**
- `pkg/commits/parser.go` — parse commit message format
- Extract type (feat, fix, refactor, docs, etc.)
- Extract scope and description
- Detect breaking changes (`BREAKING CHANGE:` footer)

**Tests:**
- `pkg/commits/parser_test.go`

---

### 2. SemVer Version Calculator
**Issue:** [#2 Core engine: SemVer parser and version bumping logic](https://github.com/GoSemantics/semrel/issues/2)

Determine the next semantic version based on analyzed commits.

**Implementation:**
- `pkg/semver/calculator.go` — calculate version bumps
- Parse current version from git tag
- Apply bump rules: major (breaking), minor (features), patch (fixes)
- Handle pre-release versions (alpha, beta, rc)

**Tests:**
- `pkg/semver/calculator_test.go`

---

### 3. Git Integration
**Issue:** [#5 Git integration: read tags, create annotated tags, push](https://github.com/GoSemantics/semrel/issues/5)

Interact with Git repositories to analyze history and manage releases.

**Implementation:**
- `pkg/git/repository.go` — Git operations
- Open existing repository
- Get last tag/version
- List commits since last tag
- Create annotated tags (optional for MVP)
- Push tags (optional for MVP)

**Tests:**
- `pkg/git/repository_test.go`

---

## Phase 2: Configuration & Changelog (HIGH PRIORITY)

### 4. Config File Parser
**Issue:** [#4 Release rules: YAML config file and branch-based rules](https://github.com/GoSemantics/semrel/issues/4)

Read and validate `.semrel.yaml` configuration.

**Implementation:**
- `pkg/config/parser.go` — load YAML config
- `pkg/config/schema.go` — validate structure
- Support `.semrel.yaml` in project root
- Provide sensible defaults
- Support environment variable overrides

**Example `.semrel.yaml`:**
```yaml
version: '1'
branches:
  - name: main
    prerelease: false
release:
  rules:
    - type: feat
      bump: minor
    - type: fix
      bump: patch
    - type: breaking
      bump: major
```

**Tests:**
- `pkg/config/parser_test.go`

---

### 5. Changelog Generator
**Issue:** [#11 Built-in plugin: changelog generator](https://github.com/GoSemantics/semrel/issues/11)

Generate changelog entries from commits.

**Implementation:**
- `pkg/changelog/generator.go` — format changelog
- Group commits by type (Features, Fixes, Breaking Changes, etc.)
- Format as markdown with links
- Support custom templates (future)

**Tests:**
- `pkg/changelog/generator_test.go`

---

## Phase 3: Release Orchestration (MEDIUM PRIORITY)

### 6. Dry-Run Mode
**Issue:** [#6 Dry-run mode implementation](https://github.com/GoSemantics/semrel/issues/6)

Implement the `semrel release --dry-run` command.

**Implementation:**
- `internal/cli/release.go` — CLI command implementation
- Analyze commits → next version
- Generate changelog preview
- Show what WOULD be done
- No actual git changes

**Output:**
```
Next version: v1.2.0 (from v1.1.0)
Type: minor (features detected)

Changelog:
## v1.2.0 (2026-05-23)

### Features
- feat: add monorepo support (#40)
- feat: implement plugin registry (#94)

### Fixes
- fix: scorecard workflow version resolution (#xxx)
```

---

### 7. Plugin Interface & Loader
**Issue:** [#8 Plugin interface: define Go interface and lifecycle hooks](https://github.com/GoSemantics/semrel/issues/8)

Define plugin system interface.

**Implementation:**
- `pkg/plugin/interface.go` — Plugin interface
- `pkg/plugin/loader.go` — Load gRPC plugins from `.semrel/`
- Support plugin lifecycle: Initialize → PreRelease → PostRelease

**Plugin Interface (Go):**
```go
type Plugin interface {
    Name() string
    Version() string
    PreRelease(context.Context, *ReleaseContext) error
    PostRelease(context.Context, *ReleaseContext) error
}
```

---

## Phase 4: Integration Tests

### 8. End-to-End Tests
**Issue:** [#21 End-to-end integration tests](https://github.com/GoSemantics/semrel/issues/21)

Test the full pipeline with real git repos.

**Implementation:**
- `tests/e2e/` — integration tests
- Create temporary git repos
- Test release pipeline
- Verify changelog, tags, version bumping

---

## Success Criteria for v0.1.0

- ✅ `semrel lint` validates commit messages
- ✅ `semrel release --dry-run` shows next version and changelog
- ✅ All HIGHEST and HIGH priority features implemented
- ✅ 80%+ code coverage
- ✅ Documentation for config format
- ✅ Example `.semrel.yaml` with comments

---

## Future Phases (Post-MVP)

- Plugin system with gRPC (Phase 3+)
- Built-in plugins: git, GitHub Releases, npm, Docker, Helm, etc. (Issues #10-36)
- Monorepo support with workspace discovery (Issues #40-43)
- Release notifications: Slack, Teams, Discord (Issues #18-19, #37-39)
- Advanced features: pre-release channels, interactive editor (Issues #45-48)

---

## References

- [ROADMAP.md](ROADMAP.md) — full project roadmap
- [ADR-001: gRPC Plugin Transport](docs/adr/ADR-001-grpc-plugin-transport.md)
- [semrel-plugins: Plugin SDK](https://github.com/GoSemantics/semrel-plugins)
