<!--
SPDX-FileCopyrightText: 2026 The semrel Authors
SPDX-License-Identifier: Apache-2.0
-->

# ADR-003: Apache License 2.0 as Project License

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-05-23 |
| **Authors** | @mwaldheim |

## Context

semrel is an open-source project with ambitions to join the CNCF ecosystem. The choice
of license affects:

- **Contributor willingness** — permissive licenses lower the barrier for corporate contributors
- **CNCF compatibility** — CNCF requires an [OSI-approved](https://opensource.org/licenses/) permissive license
- **Plugin ecosystem** — a restrictive license on the core would force all plugin authors to use the same license
- **Downstream consumption** — enterprises embedding semrel in their own tooling need clear patent and attribution terms

The main candidates considered were Apache 2.0, MIT, and GPL-2.0/3.0.

## Decision

We will license semrel (core, API definitions, documentation) under the
**Apache License, Version 2.0**.

## Consequences

**Positive:**
- Meets CNCF Sandbox and Incubating requirements (Apache 2.0 is one of the preferred CNCF licenses)
- Explicit patent grant protects users and contributors from patent litigation by contributors
- Compatible with the majority of open-source licenses (MIT, BSD, MPL-2.0)
- Allows plugin authors to choose any compatible license for their own plugins
- REUSE-compliant SPDX headers (`SPDX-License-Identifier: Apache-2.0`) are already in place

**Neutral:**
- Requires preservation of copyright notices and the `NOTICE` file in derivative works
- Slightly more verbose than MIT (the `NOTICE` file and header requirements)

**Negative:**
- Incompatible with GPL-2.0-only (compatible with GPL-3.0 via an explicit compatibility clause in GPL-3.0)

## Alternatives Considered

| License | Reason rejected |
|---------|----------------|
| **MIT** | No patent grant; does not satisfy CNCF's preference for Apache 2.0 in new projects |
| **GPL-2.0** | Copyleft would prevent corporate adoption and force plugin authors to GPL their plugins |
| **GPL-3.0** | Same copyleft concerns as GPL-2.0; incompatible with many enterprise policies |
| **MPL-2.0** | File-level copyleft is confusing for contributors; less established in the CNCF ecosystem |
