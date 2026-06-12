# Contributing to semrel

Thank you for your interest in contributing to semrel! This document provides guidelines for contributing code, documentation, and new features.

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please review our [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) before contributing.

## Getting Started

### Prerequisites

- Go 1.20+
- Git
- make (for development tasks)
- golangci-lint (for code quality)

### Development Setup

```bash
# Clone the repository
git clone https://github.com/SemRels/semrel.git
cd semrel

# Install dependencies
go mod download

# Run tests to verify setup
go test ./...

# Build the binary
make build

# Install locally
make install
```

### Project Structure

```
semrel/
├── cmd/                          # CLI entry points
│   └── semrel/main.go           # Main application
├── internal/
│   ├── cli/                     # CLI commands
│   ├── colors/                  # Terminal colors
│   └── registry/                # Plugin registry client
├── pkg/                         # Public libraries
│   ├── changelog/               # Changelog generation
│   ├── commits/                 # Conventional Commits parsing
│   ├── config/                  # Configuration loading
│   ├── git/                     # Git operations
│   ├── monorepo/                # Workspace discovery and versioning
│   ├── plugin/                  # Plugin interface and loading
│   ├── plugininstance/          # Plugin orchestration
│   ├── releasenotes/            # Release notes rendering
│   ├── sdk/                     # Plugin SDK for external developers
│   ├── semver/                  # Semantic versioning
│   └── ...                      # Other packages
├── docs/
│   ├── adr/                     # Architecture Decision Records
│   └── godoc_overview.go        # Package documentation
├── Makefile                     # Development tasks
└── .github/workflows/           # CI/CD workflows
```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/my-feature
# or
git checkout -b fix/my-fix
```

Use descriptive branch names:
- `feature/` — New functionality
- `fix/` — Bug fixes
- `docs/` — Documentation only
- `chore/` — Tooling, dependencies, etc.

### 2. Write Code with Tests

Follow these guidelines:

**Test-Driven Development (TDD):**
1. Write failing tests first
2. Implement code to pass tests
3. Refactor and improve

**File Naming:**
- Test files: `*_test.go`
- Example tests: `example_test.go`
- Internal only: package in `internal/`

**Godoc Comments:**
```go
// Package mypackage provides functionality for X.
// See: https://github.com/SemRels/semrel/issues/123
package mypackage

// MyFunc does something useful.
//
// Example:
//
//    result := MyFunc("input")
func MyFunc(s string) string {
    // Implementation
}
```

### 3. Code Quality

Before committing:

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run ./...

# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./pkg/releasenotes -v

# Check for common issues
go vet ./...
```

### 4. Commit Messages

Follow Conventional Commits:

```
type(scope): description

body (optional)

footer (optional)
```

**Types:**
- `feat` — New feature
- `fix` — Bug fix
- `docs` — Documentation
- `test` — Tests
- `chore` — Build, dependencies, tooling
- `refactor` — Code restructuring
- `perf` — Performance improvements

**Examples:**
```
feat(releasenotes): add GitHub Releases renderer

Implement RenderGitHubRelease() to format changelogs for GitHub Releases API
with emoji headers and commit SHAs.

Fixes #123
```

```
fix(monorepo): detect circular dependencies correctly

The topological sort now properly detects cycles in the dependency graph
and reports all packages involved.

See ADR-002
```

### 5. Create a Pull Request

Push your branch and open a PR:

```bash
git push origin feature/my-feature
```

In the PR description:
- Link related issues: `Fixes #123` or `Relates to #456`
- Describe what changed and why
- Include testing instructions if relevant
- Reference ADRs if this is a design decision

**PR Checklist:**
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Godoc comments added for public types/functions
- [ ] No breaking changes (or documented in BREAKING.md)
- [ ] Follows code style (gofmt, golangci-lint)

## Adding Features

### Feature: New Changelog Renderer

**Steps:**
1. Add `Render*()` method to `releasenotes.ReleaseNotes` in `pkg/releasenotes/`
2. Write tests in `*_test.go`
3. Document in Godoc comments
4. Add ADR if it's a significant decision
5. Update `docs/README.md` with examples

**Example:**
```go
// pkg/releasenotes/myformat.go
func (r *ReleaseNotes) RenderMyFormat() string {
    // Implementation using r.Version, r.Date, r.Breaking, r.Features, etc.
}
```

### Feature: New Plugin Type

**Steps:**
1. Create new plugin repository in semrel-plugins organization
2. Use [plugin-template](../plugin-template/) as starting point
3. Implement `sdk.Plugin` interface
4. Write tests
5. Document in README
6. Register in [semrel-registry](../semrel-registry/)

**Plugin SDK Usage:**
```go
import "github.com/SemRels/semrel/pkg/sdk"

type MyPlugin struct{}

func (p *MyPlugin) Execute(ctx context.Context, event sdk.ReleaseEvent) (*sdk.Result, error) {
    // Validate input
    if event.Version == "" {
        return nil, fmt.Errorf("version required")
    }
    
    // Perform action
    
    // Return results
    return &sdk.Result{
        Outputs: map[string]string{
            "output_key": "output_value",
        },
    }, nil
}
```

### Feature: New Configuration Option

**Steps:**
1. Add field to `config.Config` or relevant struct
2. Add YAML/TOML/JSON tags
3. Add validation in config loader
4. Update `.semrel.yaml` examples
5. Document in `docs/README.md`
6. Add tests

**Example:**
```go
// pkg/config/config.go
type Config struct {
    // ...existing fields...
    
    // NewOption describes the new feature
    NewOption string `yaml:"newOption,omitempty" toml:"newOption" json:"newOption,omitempty"`
}
```

### Feature: New Command or Subcommand

**Steps:**
1. Add command struct in `internal/cli/`
2. Implement Execute() method
3. Register in root command
4. Add tests in `*_test.go`
5. Document in README.md and help text
6. Add integration tests if applicable

**Example:**
```go
// internal/cli/mycommand.go
type MyCommand struct {
    // Fields for options
}

func (c *MyCommand) Execute(ctx context.Context, opts *Options) error {
    // Implementation
    return nil
}
```

## Writing Tests

### Unit Tests

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test"
    expected := "result"
    
    // Act
    actual := MyFunction(input)
    
    // Assert
    if actual != expected {
        t.Errorf("expected %q, got %q", expected, actual)
    }
}
```

### Table-Driven Tests

```go
func TestParsing(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    interface{}
        wantErr bool
    }{
        {"valid", "feat: message", Entry{Type: "feat"}, false},
        {"empty", "", Entry{}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("wantErr %v, got %v", tt.wantErr, err)
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("want %+v, got %+v", tt.want, got)
            }
        })
    }
}
```

### Integration Tests

```go
func TestReleaseIntegration(t *testing.T) {
    // Create temporary git repo
    tmpdir := t.TempDir()
    repo, _ := git.InitRepo(tmpdir)
    
    // Create commits
    repo.Commit("feat: initial release")
    
    // Run release
    cmd := &ReleaseCommand{}
    result, _ := cmd.Execute(ctx, opts)
    
    // Verify results
    if result.NextVersion != "v0.1.0" {
        t.Errorf("unexpected version: %s", result.NextVersion)
    }
}
```

### Mocking and Fixtures

Use interfaces and dependency injection:

```go
// Mockable interface
type GitClient interface {
    Tag(ctx context.Context, name, message string) error
    Push(ctx context.Context) error
}

// Mock implementation for tests
type MockGitClient struct {
    TagCalled bool
}

func (m *MockGitClient) Tag(ctx context.Context, name, message string) error {
    m.TagCalled = true
    return nil
}
```

## Documentation

### Godoc

All public types and functions must have Godoc comments:

```go
// Package mypackage provides functionality for X.
//
// # Overview
//
// Detailed explanation of the package purpose.
//
// # Usage
//
//    import "github.com/SemRels/semrel/pkg/mypackage"
//
//    result := mypackage.Do(ctx, input)
//
// See: https://github.com/SemRels/semrel/issues/123
package mypackage

// MyFunc does something important.
//
// It accepts X and returns Y. If condition Z occurs, it returns an error.
//
// Example:
//
//    result, err := MyFunc("input")
//    if err != nil {
//        log.Fatal(err)
//    }
func MyFunc(s string) (string, error) {
    // Implementation
}
```

### Architecture Decision Records (ADRs)

Use ADRs to document significant decisions:

```markdown
# ADR-NNN: Title

**Date:** 2026-06-12  
**Status:** Proposed/Accepted/Superseded  
**Context:** Which phase/version this relates to

## Problem

What problem does this solve?

## Decision

What did we decide?

## Rationale

Why this decision?

## Consequences

What are the positive and negative consequences?
```

## Code Review Process

### For Contributors

- Keep PRs focused (one feature or fix per PR)
- Respond to review feedback promptly
- Mark conversations as resolved after addressing them
- Request re-review when changes are made

### For Reviewers

- Review for correctness, style, and maintainability
- Ask questions to understand intent
- Suggest improvements, not demands
- Approve when satisfied

## Release Process

Releases follow Semantic Versioning and are automated via semrel:

1. Features go to `develop` branch (triggers `v0.X.0-alpha.N`)
2. When ready for testing, merge to `next` (triggers `v0.X.0-beta.N`)
3. When stable, merge to `main` (triggers `v0.X.0`)

Releases are created automatically by the CI workflow.

## Getting Help

- **Questions?** Ask in GitHub Discussions
- **Bug Reports?** Open a GitHub Issue
- **Security Issues?** Email security@semrel.dev (follow SECURITY.md)
- **Chat?** Join our Matrix room

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License. See [LICENSE](../LICENSE) for details.

SPDX: Apache-2.0

---

Thank you for contributing to semrel! 🎉
