# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for `semrel`.

ADRs capture significant technical decisions, the context in which they were made,
and their consequences. Each ADR is immutable once accepted; superseded ADRs are
marked accordingly.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-001](ADR-001-grpc-plugin-transport.md) | Out-of-process plugin transport via hashicorp/go-plugin + gRPC | Accepted |

## Format

ADRs follow the [Michael Nygard template](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions):
`Title`, `Status`, `Context`, `Decision`, `Consequences`.

## Creating a new ADR

1. Copy `ADR-000-template.md` to `ADR-NNN-short-title.md`
2. Fill in all sections
3. Open a PR — ADR status starts as `Proposed`
4. After merge the status moves to `Accepted`

