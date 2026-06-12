# Semantic Versioning & Release Strategy

This document explains how semrel manages versions, pre-releases, and maintenance releases.

## Semantic Versioning Basics

semrel follows [Semantic Versioning 2.0.0](https://semver.org/):

```
v1.2.3-alpha.1+build.metadata
└┬┘ └┬┘ └┬┘ └────┬────┘ └──┬────┘
 │   │   │       │          └─ Build metadata (ignored in precedence)
 │   │   │       └────────────── Pre-release version
 │   │   └───────────────────── Patch version (backwards-compatible bug fixes)
 │   └─────────────────────── Minor version (new features, backwards-compatible)
 └───────────────────────── Major version (breaking changes)
```

**Rules:**
- MAJOR: Breaking changes (bump when incompatible API changes)
- MINOR: New features (backwards-compatible)
- PATCH: Bug fixes (backwards-compatible)
- Pre-release: alpha, beta, rc (lower precedence than release)
- Build metadata: Ignored in version precedence

**Precedence Examples:**
```
v1.0.0-alpha.1 < v1.0.0-alpha.2 < v1.0.0-beta.1 < v1.0.0-rc.1 < v1.0.0
v1.0.0 < v1.1.0 < v2.0.0
v2.0.0 < v2.0.1
```

## Conventional Commits to Semver Mapping

semrel uses [Conventional Commits](https://www.conventionalcommits.org/) to automatically determine version bumps:

| Conventional Commit | Semver Bump | Example |
|---|---|---|
| `feat(scope): ...` | MINOR | v1.0.0 → v1.1.0 |
| `fix(scope): ...` | PATCH | v1.0.0 → v1.0.1 |
| `BREAKING CHANGE: ...` | MAJOR | v1.0.0 → v2.0.0 |

**Bump Rules Configuration (`.semrel.yaml`):**

```yaml
rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: perf
    bump: minor          # Performance improvements also bump minor
  - type: refactor
    bump: patch
  - type: breaking
    bump: major
  - default: patch       # Any other type gets patch bump
```

**Algorithm:**
1. Collect all commits since last tag
2. Parse each commit (type, scope, description, breaking?)
3. Find highest bump level (major > minor > patch > none)
4. Apply bump: v1.2.3 + minor = v1.3.0

## Pre-Release Channels

semrel supports multiple pre-release channels (alpha, beta, rc) per branch:

**Configuration:**
```yaml
branches:
  - name: develop
    prerelease: alpha
  - name: next
    prerelease: beta
  - name: main              # No prerelease = final release
```

**Release Examples:**

For a project starting at v0.1.0:

| Branch | Commits | New Tag | Notes |
|--------|---------|---------|-------|
| main | feat | v0.2.0 | Final release |
| next | feat | v0.2.0-beta.1 | Beta channel |
| next | feat | v0.2.0-beta.2 | Auto-incremented |
| develop | feat | v0.2.0-alpha.1 | Alpha channel |
| develop | fix | v0.2.0-alpha.2 | Auto-incremented |

**Pre-Release Numbering:**
- Counter auto-increments per version & channel
- v0.2.0-alpha.1, v0.2.0-alpha.2, ... then v0.2.0-beta.1
- Persisted in git tags and version files

## Branch Management

### Main Release Branch

The `main` branch is the source of production releases:

```
main:  v1.0.0 ← v1.1.0 ← v1.2.0 ← v1.2.1 ← v2.0.0 (stable releases)
                                      ↑
                                  Fix branch
                                  (hotfix)
```

### Pre-Release Branches

Pre-release branches allow testing before merging to `main`:

```
develop:  v1.2.0-alpha.1 ← v1.2.0-alpha.2 ← v1.2.0-alpha.3
           ↓ (merge to next when ready for beta)
next:     v1.2.0-beta.1 ← v1.2.0-beta.2
           ↓ (merge to main when stable)
main:                        v1.2.0
```

**Typical Workflow:**
1. Features land on `develop`
2. Nightly/Weekly releases from `develop` for testing
3. Merge `develop` → `next` for beta testing
4. Merge `next` → `main` for stable release

### Maintenance Branches

For maintaining older major versions:

```yaml
branches:
  - name: main
  - name: "1.x"
    maintenance: true
  - name: "2.x"
    maintenance: true
```

Allows releases for older versions:

```
main (v2.x):        v2.0.0 ← v2.1.0 ← v2.2.0 (current major version)
                                        ↓
1.x (v1.x):  v1.0.0 ← v1.1.0 ← v1.2.0 ← v1.2.1 (maintenance)
             (no longer primary focus, bug fixes only)
```

## Release Configuration Examples

### Minimal (Single Stable Channel)

```yaml
schemaVersion: 1
branches:
  - name: main
```

All commits trigger releases (major/minor/patch based on commit type).

### With Pre-Release Channels

```yaml
schemaVersion: 1
branches:
  - name: develop
    prerelease: alpha
  - name: next
    prerelease: beta
  - name: main
```

Releases cascade from develop → next → main.

### With Maintenance Releases

```yaml
schemaVersion: 1
branches:
  - name: main              # v2.x (current)
  - name: "1.x"             # v1.x (maintenance)
    maintenance: true
rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: breaking
    bump: major
```

Maintenance branch only accepts patches (bug fixes).

## Version Ceiling

Limit releases to a maximum version:

```yaml
versionCeiling: "1.x"      # Never release v2.0.0 or higher
ceilingStrategy: "error"   # "error", "warn", or "skip"
```

Useful for:
- Preventing accidental major version bumps
- Enforcing internal freeze (pre-release embargo)
- Testing version ceiling handling

## Force Bumping

Override automatic version detection:

```bash
semrel release --force-bump-major     # v1.2.3 → v2.0.0
semrel release --force-bump-minor     # v1.2.3 → v1.3.0
semrel release --force-bump-patch     # v1.2.3 → v1.2.4
```

Use cases:
- Emergency releases
- Manual override for complex scenarios
- Testing

## Tag Management

### Tag Prefix

Configure tag prefix in `.semrel.yaml`:

```yaml
tagPrefix: "v"             # Default: "v"
                           # Results: v1.0.0, v1.1.0, etc.
```

Or per-package in monorepo:

```yaml
workspace:
  packages:
    - name: core
      path: packages/core
      tagPrefix: "core@"   # Results: core@v1.0.0
```

### Existing Tag Handling

Control behavior if tag already exists:

```yaml
tagExistsStrategy: "error"  # "error" (default), "overwrite", or "skip"
```

## Monorepo Versioning

### Independent Versioning

Each package has its own version:

```
packages/core/     v1.2.0 ← v1.3.0 ← v1.3.1
packages/api/      v2.0.0 ← v2.1.0
packages/web/      v0.5.0 ← v0.6.0
```

Tags: `core@v1.3.1`, `api@v2.1.0`, `web@v0.6.0`

**Configuration:**
```yaml
workspace:
  strategy: independent
  packages:
    - name: core
      path: packages/core
    - name: api
      path: packages/api
    - name: web
      path: packages/web
```

**Behavior:**
- Each package releases independently
- Version number based on changes in that package only
- Can be released in parallel (if dependencies allow)

### Lockstep Versioning

All packages share one version:

```
packages/core/     v1.0.0 ← v1.1.0 ← v1.2.0
packages/api/      v1.0.0 ← v1.1.0 ← v1.2.0
packages/web/      v1.0.0 ← v1.1.0 ← v1.2.0
```

Tags: `v1.2.0` (shared)

**Configuration:**
```yaml
workspace:
  strategy: lockstep
  packages:
    - name: core
      path: packages/core
    - name: api
      path: packages/api
    - name: web
      path: packages/web
```

**Behavior:**
- One version for entire monorepo
- Version bump = max bump across all packages
- Simplifies coordination but less flexible

## Version Ceiling Examples

### Prevent Breaking Changes

```yaml
versionCeiling: "0.x"
ceilingStrategy: "error"
```

Never releases v1.0.0 (prevents accidental graduation).

### Limit Major Version

```yaml
versionCeiling: "1.x"
```

Allows: v1.0.0, v1.1.0, v1.2.0, etc.
Prevents: v2.0.0, v2.1.0, etc.

## Pre-Release Scenarios

### Scenario 1: Feature → Beta → Release

```
1. Developer commits feat on develop
   Result: v1.2.0-alpha.1

2. Developer commits another feat
   Result: v1.2.0-alpha.2

3. Merge develop → next (merge commit)
   Result: v1.2.0-beta.1

4. Test in beta, merge next → main
   Result: v1.2.0 (final release)

5. Next feature on develop
   Result: v1.3.0-alpha.1 (new minor cycle)
```

### Scenario 2: Emergency Hotfix

```
main:  v1.2.0 ← (bug found)
       ↓ (create hotfix branch from main)
hotfix: v1.2.1-rc.1 ← (test)
        ↓ (merge to main)
main:   v1.2.1 (released)

develop: v1.3.0-alpha.1 ← (merge v1.2.1 back)
```

### Scenario 3: Parallel Development

```
main:      v1.0.0 ← v1.1.0 (stable releases)
           
next:                v1.2.0-beta.1 ← v1.2.0-beta.2
                     (v1.2.0 in progress)
           
develop:                              v1.2.0-alpha.1 ← v1.3.0-alpha.1
                                      (v1.2.0 and v1.3.0 in parallel development)
```

## Troubleshooting

### Version Not Incrementing

**Symptom:** Release always produces same version.

**Causes:**
- No commits since last tag
- Commits don't follow Conventional Commits format
- `--force-bump-*` not used

**Solution:**
```bash
# Check commits since last tag
git log v1.2.0..HEAD --oneline

# Ensure commits are Conventional Commits
git log --format=%B v1.2.0..HEAD | head -20

# Force bump if needed
semrel release --force-bump-patch
```

### Wrong Version Calculated

**Symptom:** Got v1.1.0 but expected v1.2.0.

**Causes:**
- Commit message doesn't match rules
- Custom rules override defaults
- Incorrect version ceiling

**Solution:**
```bash
# Check configured rules
cat .semrel.yaml | grep -A 10 "^rules:"

# Check commit format
git log -1 --format=%B

# Test with --dry-run
semrel release --dry-run
```

### Tag Already Exists

**Symptom:** Release fails because tag exists.

**Causes:**
- Duplicate release attempt
- Force bump to same version
- Manual tag created

**Solution:**
```yaml
# Option 1: Allow overwriting
tagExistsStrategy: "overwrite"

# Option 2: Skip if exists
tagExistsStrategy: "skip"

# Option 3: Bump version and retry
semrel release --force-bump-patch
```

## Best Practices

1. **Enforce Conventional Commits** — Use commitlint or pre-commit hooks
2. **Use Pre-Release Branches** — Test before stable release
3. **Document Breaking Changes** — In commit footer: `BREAKING CHANGE:`
4. **Keep Main Stable** — Only merge tested code
5. **Review Release Plans** — Use `--dry-run` before releasing
6. **Monitor Pre-Releases** — Gather feedback before GA
7. **Automate** — Let semrel handle bumping, use CI/CD

## Related Documentation

- [Semantic Versioning Specification](https://semver.org/)
- [Conventional Commits Specification](https://www.conventionalcommits.org/)
- [semrel Configuration Reference](./README.md#configuration-reference)
- [ADR-004: Pre-Release Channels](./adr/ADR-001-to-008.md#adr-004-pre-release-channels)

