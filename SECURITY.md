# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| `main` (pre-release) | ✅ |

Once stable releases are published this table will list supported version ranges.

## Reporting a Vulnerability

**Please do not open a public GitHub Issue for security vulnerabilities.**

Report security issues privately via **[GitHub Security Advisories](https://github.com/SemRels/semrel/security/advisories/new)**.

You can also reach the maintainers at the addresses listed in [MAINTAINERS.md](MAINTAINERS.md).

### What to include

- Description of the vulnerability and its potential impact
- Steps to reproduce or a proof-of-concept (if available)
- Affected versions
- Any suggested mitigations

### Response SLA

| Action | Target |
|--------|--------|
| Initial acknowledgement | 48 hours |
| Status update | 7 days |
| Patch / mitigation | 90 days |

We follow responsible disclosure. We will coordinate a public disclosure date with you.

## CVE Process

semrel may explore applying for CNCF Sandbox status in the future. No application has been made. This section will be updated if that changes.

## Security Audits

No formal security audit has been conducted yet. This section will be updated when one is completed.

## Artifact Signing and Verification

Starting from `v0.1.0`, semrel release artifacts are signed using
[Sigstore Cosign](https://github.com/sigstore/cosign) (keyless OIDC signing).

### Verifying a release artifact

```bash
# Download the artifact and its bundle sidecar
curl -LO https://github.com/SemRels/semrel/releases/download/v1.0.0/semrel_linux_amd64.tar.gz
curl -LO https://github.com/SemRels/semrel/releases/download/v1.0.0/semrel_linux_amd64.tar.gz.bundle

# Verify the signature (requires cosign v2+)
cosign verify-blob \
  --bundle semrel_linux_amd64.tar.gz.bundle \
  --certificate-identity-regexp "https://github.com/SemRels/semrel/.github/workflows/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  semrel_linux_amd64.tar.gz
```

### SBOM

SBOMs (Software Bill of Materials) are published in both CycloneDX 1.4 (JSON) and
SPDX 2.3 (tag-value) formats alongside every release.

### SLSA Provenance

SLSA Level 1 build provenance is published with each release, documenting the
build inputs and artifact digests.

## Signing Key Identity

Keyless Cosign signing uses the GitHub Actions OIDC identity:
- **Issuer:** `https://token.actions.githubusercontent.com`
- **Subject pattern:** `https://github.com/SemRels/semrel/.github/workflows/release.yml@refs/tags/v*`

