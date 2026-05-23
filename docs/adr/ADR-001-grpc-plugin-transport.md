# ADR-001: Out-of-process plugin transport via hashicorp/go-plugin + gRPC

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-05-22 |
| **Authors** | @mwaldheim @tboerger |

## Context

`go-semrel` needs an extensible plugin system that satisfies the following
requirements:

- **Language independence** – plugin authors must be able to use any language
  that can compile a binary and speak gRPC (Go, Rust, Python via grpcio, etc.)
- **Isolation** – a misbehaving plugin must not crash the host process
- **Security** – plugins run with their own filesystem and process boundaries;
  no shared memory between host and plugin
- **Air-gapped deployments** – plugins must be resolvable from a local directory
  without network access
- **Versioned contracts** – the RPC interface must be independently versioned so
  host and plugins can evolve separately
- **Windows compatibility** – the transport layer must work on Linux, macOS and
  Windows without custom abstractions

The original Node.js `semantic-release` uses in-process JavaScript modules loaded
via `require()`.  A direct port of this model to Go would limit plugins to Go and
make crash isolation impossible.

Two primary in-process Go plugin mechanisms were evaluated and rejected (see
[Alternatives Considered](#alternatives-considered)).

## Decision

We will use [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) as the
plugin host library with **gRPC** as the transport protocol.

### Architecture overview

```
┌─────────────────────────────────────────────────┐
│  go-semrel host process                          │
│                                                  │
│  ┌────────────┐     gRPC over        ┌────────┐ │
│  │ PluginMgr  │◄───UDS / NamedPipe──►│ Plugin │ │
│  └────────────┘                      │ binary │ │
│        │                             └────────┘ │
│  ┌─────▼──────┐   one child process             │
│  │ go-plugin  │   per plugin instance            │
│  │  Client    │                                  │
│  └────────────┘                                  │
└─────────────────────────────────────────────────┘
```

- Each plugin is a **separate binary** that implements the gRPC server side of
  the plugin contract defined in `api/proto/v1/semantic_release.proto`.
- The host launches plugin binaries as **child processes** via `go-plugin`.
  Communication uses Unix Domain Sockets on Linux/macOS and Named Pipes on
  Windows — both handled transparently by `go-plugin`.
- **stdout** is reserved exclusively for the `go-plugin` handshake (`1|1|tcp|…`
  or the UDS equivalent).  Plugin authors MUST NOT write anything to stdout.
  All log output MUST go to **stderr**.
- The host validates the plugin's magic cookie before establishing the gRPC
  channel, preventing accidental execution of arbitrary binaries.

### Plugin categories

The proto contract exposes the following service groups, each corresponding to a
plugin category:

| Category | Interface | Purpose |
|----------|-----------|---------|
| `Provider` | `ProviderPlugin` | Abstract VCS operations (create release, upload assets, read commits) |
| `CICondition` | `CIConditionPlugin` | Verify the environment is a CI system authorised to release |
| `CommitAnalyzer` | `CommitAnalyzerPlugin` | Parse commits and determine the next version bump |
| `ChangelogGenerator` | `ChangelogGeneratorPlugin` | Render release notes in a target format |
| `FilesUpdater` | `FilesUpdaterPlugin` | Update version strings in project files before tagging |
| `Hooks` | `HooksPlugin` | `OnSuccess` / `OnFail` lifecycle callbacks (notifications, etc.) |

### Plugin discovery

Plugins are resolved in the following order:

1. **Local directory** – `.semrel/<GOOS>_<GOARCH>/<plugin-name>/<version>/<binary>`
   (air-gapped / CI pre-cache friendly)
2. **Plugin registry** – `https://registry.go-semantic-release.xyz` (automatic
   download, verified checksum)

The local directory takes precedence, enabling fully offline deployments.

### Versioning

The gRPC service is versioned through the Protobuf package name
(`semantic_release.v1`).  Breaking changes require a new package version
(`v2`, etc.) and a new `go-plugin` protocol number, ensuring backward
compatibility during transition.

## Consequences

### Positive

- Plugin authors are not restricted to Go — any language with gRPC support works.
- Plugin crashes are isolated to the child process; the host catches the error and
  continues (or aborts cleanly).
- The gRPC contract is self-documenting via `.proto` files checked into the repo.
- `go-plugin` handles TLS, mTLS, and the OS-specific transport abstraction.
- Air-gapped deployments are a first-class concern.

### Negative

- Each plugin invocation spawns a child process — there is process-startup
  overhead (~10–50 ms per plugin binary on cold start).  Mitigated by keeping
  plugin binaries resident for the duration of a release run.
- Plugin authors must implement a gRPC server, which is more complex than a
  simple Go `interface{}`.  Mitigated by the **Plugin SDK** (tracked in a
  separate issue) that provides language-specific boilerplate generators.
- Debugging requires attaching to two processes instead of one.

### Neutral

- `go-plugin` is maintained by HashiCorp and battle-tested in Terraform,
  Vault, and Packer — it is a well-understood dependency.
- The proto file becomes a formal API boundary and must be versioned carefully.

## Alternatives Considered

### Go `plugin` package (`.so` shared libraries)

The standard library `plugin` package loads `.so` files into the host process.

**Rejected** because:
- Only works on Linux and macOS — no Windows support.
- Shared library ABI is not stable across Go compiler versions.
- A panic inside a plugin brings down the entire host process.
- Build toolchain for plugin authors must exactly match the host's Go version.

### In-process interface registration (function pointers / `interface{}`)

Plugins register themselves at init-time via a global registry (similar to
`database/sql` drivers).

**Rejected** because:
- Requires plugins to be written in Go and compiled into the same binary.
- No crash isolation.
- No versioning boundary — a breaking change in the host interface requires all
  plugins to be recompiled simultaneously.
