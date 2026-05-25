<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: 2026 The semrel Authors
-->

# CNCF Due Diligence — semrel

> Prepared for CNCF Sandbox application.
> Template: https://github.com/cncf/toc/blob/main/process/due-diligence-guidelines.md

---

## 1. Project Overview

**Name:** semrel (go-semrel)

**Repository:** https://github.com/SemRels/semrel

**Website / Docs:** https://github.com/SemRels/semrel/tree/main/docs

**License:** Apache 2.0

**Language:** Go

**Summary:**
semrel is a Go-based semantic versioning and release automation system with a
plugin architecture. It automates the full release lifecycle — from conventional
commit parsing through version calculation, changelog generation, and multi-target
publishing (GitHub Releases, NuGet, PyPI, Homebrew, OCI registries, Terraform
registry, Gitea, and more). Designed with first-class support for monorepos and
supply-chain security (SBOM, SLSA provenance, Cosign signing).

---

## 2. Alignment with CNCF Mission

semrel addresses the release automation gap in cloud-native CI/CD pipelines:

- **Cloud native lifecycle**: Automates release workflows for projects hosted on
  cloud-native infrastructure (GitHub Actions, Gitea CI, Forgejo).
- **Supply chain security**: Generates CycloneDX/SPDX SBOMs, SLSA provenance
  documents, and Cosign-signed artifacts — directly supporting CNCF supply-chain
  security initiatives (SLSA, OpenSSF).
- **Ecosystem integration**: Native support for OCI registries (ORAS), Helm,
  Terraform registries, and container image signing.
- **Open standards**: Follows Conventional Commits, Keep a Changelog,
  Semantic Versioning, REUSE/SPDX, and OpenSSF Best Practices.

---

## 3. Project Status

### Maturity

| Dimension | Status |
|-----------|--------|
| Development stage | Pre-alpha / active development |
| First commit | 2025 |
| Current version | 0.x (pre-release) |
| Breaking changes | Possible until v1.0 |
| Stability commitment | Semver after v1.0 |

### Community

| Metric | Value |
|--------|-------|
| Primary maintainer | @mwaldheim |
| Contributors | See [MAINTAINERS.md](../MAINTAINERS.md) |
| Public adopters | See [ADOPTERS.md](../ADOPTERS.md) |
| Governance model | See [GOVERNANCE.md](../GOVERNANCE.md) |

### Activity (recent)

- Active feature development targeting v0.0.1 milestone
- CI runs on every PR across Go 1.24, 1.25, 1.26
- REUSE/SPDX compliance enforced on every PR

---

## 4. Governance and Community Health

- **Governance:** Documented in [GOVERNANCE.md](../GOVERNANCE.md) — BDFL model
  transitioning to committee-based at v1.0.
- **Code of Conduct:** [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) (Contributor Covenant v2.1)
- **Contributing guide:** [CONTRIBUTING.md](../CONTRIBUTING.md)
- **Decision records:** [docs/adr/](adr/) — key architectural decisions documented as ADRs
- **Maintainers:** [MAINTAINERS.md](../MAINTAINERS.md)
- **Roadmap:** [ROADMAP.md](../ROADMAP.md) — public roadmap with milestone tracking

---

## 5. Security Posture

- **Security policy:** [SECURITY.md](../SECURITY.md) — responsible disclosure via GitHub
  Security Advisories
- **Supply chain:**
  - REUSE/SPDX license headers on all source files
  - DCO (Developer Certificate of Origin) required on all commits
  - Cosign signing for release artifacts (see [pkg/signing](../pkg/signing/))
  - CycloneDX 1.4 and SPDX 2.3 SBOM generation (see [pkg/sbom](../pkg/sbom/))
  - SLSA Level 1 provenance via [pkg/signing](../pkg/signing/)
- **OpenSSF Scorecard:** Tracked at https://scorecard.dev/viewer/?uri=github.com/SemRels/semrel
- **OpenSSF Best Practices badge:** Tracked at https://bestpractices.coreinfrastructure.org
- **Known vulnerabilities / CVE history:** None to date (pre-release project)
- **Security audit:** Not yet conducted; planned before v1.0 GA

---

## 6. License Compliance

- **License:** Apache 2.0 (OSI-approved, CNCF-compatible)
- **Compliance tooling:** [REUSE](https://reuse.software/) compliance enforced in CI
- **Third-party dependencies:** All vendored/Go module dependencies reviewed for
  license compatibility in `go.mod`/`go.sum`

---

## 7. Cloud Native Landscape Fit

semrel complements existing CNCF projects:

| Existing project | Relationship |
|------------------|--------------|
| **Flux** | semrel can trigger Flux image policy updates after a release |
| **Argo CD** | semrel creates the tagged release that Argo CD deploys |
| **sigstore/cosign** | semrel integrates cosign for artifact signing (pkg/signing) |
| **SLSA framework** | semrel generates SLSA Level 1 provenance documents |
| **OCI / ORAS** | semrel can push artifacts to OCI registries via ORAS |

semrel does **not** overlap with:
- Container image builders (Buildpacks, ko, Docker) — semrel only orchestrates releases
- GitOps controllers — semrel is the *source* of releases, not the deployer

---

## 8. Vendor Neutrality

- Supports GitHub, GitLab, Gitea, and Forgejo release targets
- Plugin architecture allows community plugins for any platform
- No hard dependency on any single cloud provider's services
- OIDC-based keyless signing (Sigstore) works with any OIDC provider

---

## 9. Gaps and Planned Improvements

| Gap | Plan |
|-----|------|
| No v1.0 stable release | Roadmap targets v1.0 after full feature set |
| Single primary maintainer | Seeking co-maintainers from early adopters |
| No formal security audit | Planned before v1.0 |
| OpenSSF badge not yet at Passing | Working through criteria (see #73) |
| CLOMonitor signed-releases check | cosign integration in progress (#80) |
