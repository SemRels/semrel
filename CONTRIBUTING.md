# Contributing to semrel

Thank you for your interest in contributing! 🎉

## Prerequisites

- Go 1.23+
- `git` with commit signing configured
- `golangci-lint` for linting (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

## Developer Certificate of Origin (DCO)

All commits **must** include a `Signed-off-by` trailer. This certifies you agree to the [Developer Certificate of Origin](https://developercertificate.org/).

```bash
git commit -s -m "feat: my new feature"
# or configure globally:
git config --global format.signoff true
```

PRs with unsigned commits will be blocked by the DCO check.

## Workflow

1. Fork the repository and clone it locally
2. Create a branch: `git checkout -b feat/<issue-number>-short-description`
3. Make your changes — keep commits small and focused
4. Write or update tests (`go test ./...`)
5. Run the linter (`make lint`)
6. Push and open a PR targeting `main`

## Branch Strategy

| Branch | Purpose |
|--------|---------|
| `main` | Protected. Only accepts PRs with passing CI, DCO, and at least one review. No direct pushes. |
| `feat/<n>-slug` | Feature / fix branches. Branch from `main`, PR back to `main`. |
| `release/N.x` | Maintenance branches for backport releases. Branch from the release tag. |

Required status checks on `main`: `CI / Test`, `DCO`, `REUSE`, `Scorecard`.
All commits on `main` must be signed (GPG / SSH / Vigilant mode).
Merge strategy: squash-merge or rebase — no merge commits.

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

[optional body]

[optional footers]
Signed-off-by: Your Name <you@example.com>
```

**Types:** `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `chore`, `ci`, `revert`

Breaking changes: append `!` to the type or add `BREAKING CHANGE:` footer.

## Pull Request Checklist

- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] All commits are signed off (`git commit -s`)
- [ ] Relevant docs updated
- [ ] Issue referenced in PR description (`Closes #<number>`)

## Architecture Decision Records

Significant technical decisions are documented as [Architecture Decision Records](docs/adr/README.md) in `docs/adr/`.
Before making a large architectural change, check whether an existing ADR covers the area.
For new architectural decisions, open a PR that adds a new ADR following the [template](docs/adr/ADR-000-template.md).

## Code Style

- Standard Go formatting (`gofmt`)
- All exported symbols must have godoc comments
- Add `// SPDX-License-Identifier: Apache-2.0` and `// SPDX-FileCopyrightText:` headers to new files

## Reporting Issues

Use the [GitHub Issue templates](.github/ISSUE_TEMPLATE/) for bug reports and feature requests. For security vulnerabilities, follow [SECURITY.md](SECURITY.md).

