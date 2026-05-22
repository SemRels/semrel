<!--
SPDX-FileCopyrightText: 2026 The go-semrel Authors
SPDX-License-Identifier: Apache-2.0
-->

# ADR-002: Repository Separation — Why go-semrel Itself Uses Multiple Repos (Not a Monorepo)

**Status:** Accepted  
**Date:** 2026-05-23  
**Deciders:** go-semrel maintainers  
**Affected Parties:** Contributors, plugin developers, library consumers  

## Context

During project inception, go-semrel was structured as a monorepo containing:
- Core CLI and release engine (`cmd/`, `internal/`)
- gRPC API and Protocol Buffers (`api/proto/v1/`)
- Plugin SDK and built-in plugins (`plugins/`)
- Shared libraries (`pkg/` — changelog, commits, semver)

### Initial Reasoning
- Easier rapid prototyping and initial coordination
- Single CI/CD pipeline for feedback loops
- Shared legal and governance groundwork

### Problems with Monorepo Approach
1. **Unclear ownership:** Code is together but responsibility isn''t
2. **Entangled CI:** Unrelated changes trigger expensive builds
3. **Release coupling:** API bump forces CLI release even without CLI changes
4. **Plugin developer friction:** External plugin developers must clone entire monorepo
5. **Version confusion:** Is `v1.2.3` the CLI, the API, or plugins?
6. **Registry complexity:** Multiple artifacts with same version
7. **Dependency graph obscurity:** Unclear what imports what and why

## Decision

**Split go-semrel into four separate, independently-versioned repositories:**

| Repo | Purpose | Root Module |
|------|---------|-------------|
| **go-semrel** | CLI binary, release orchestration | `github.com/GoSemantics/go-semrel` |
| **go-semrel-api** | gRPC API, Protocol Buffers | `github.com/GoSemantics/go-semrel-api` |
| **go-semrel-plugins** | Plugin SDK library | `github.com/GoSemantics/go-semrel-plugins` |
| **go-semrel-lib** | Public libraries (changelog, commits, semver) | `github.com/GoSemantics/go-semrel-lib` |

### Key Principles
1. **Independent versioning:** API v2 ≠ CLI v2
2. **Decoupled releases:** Plugins ship independently
3. **Clear boundaries:** Import only what you need
4. **Clearer ownership:** Each repo has clear scope

## Important: Monorepo Support as a FEATURE

**This does NOT affect end-user monorepo support!**

Issues #40–43 remain fully in scope:
- #40: Workspace/package discovery
- #41: Independent versioning per package  
- #42: Synchronized versioning mode
- #43: Inter-package dependency graphs

These are FEATURES for go-semrel users, not architectural constraints on go-semrel itself.

## Consequences

### Positive ✅
- Faster CLI iteration without API approval
- Plugin SDK evolves independently
- CI/CD parallelization
- Clearer responsibility and code ownership
- Easier onboarding for maintainers

### Negative ⚠️
- More coordination for breaking changes
- Multiple repos to clone locally
- Initial migration effort

### Mitigations
- Coordinated versioning scheme (initially aligned)
- Makefile scripts for multi-repo ops
- Clear dependency pinning in go.mod
- Cross-repo issue coordination

## Related

- **ADR-001:** Out-of-process plugin transport (gRPC)
- **ROADMAP.md:** v0.1.5 milestone
- **Issues #40–43:** End-user monorepo support
