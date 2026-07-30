# Audience Registry

Single source of truth for project audiences and their artifact needs. Derived from the
[SPARK analysis](../inception/spark-analysis.md), 2026-07-30.

## Audiences

| ID | Audience | Category | Needs | Derived artifacts |
|----|----------|----------|-------|-------------------|
| A1 | Enterprise IaC / platform engineer | Primary | Reproducible control planes; lockfile; drift report; dependable idempotency across time | Tutorial, How-to (lock and verify a controller), C4:Person |
| A2 | OEM delivery team | Primary | xBOM for delivered infrastructure — the Ansible content is part of its provenance chain | How-to (produce a BOM for a delivery), Reference (output schema) |
| A3 | Packager (RPM/deb, internal mirror, OEM bundle) | Primary | Authoritative component list with versions, licences, hashes for what is redistributed | Reference (identity and purl construction), Explanation (identity under repackaging) |
| A4 | SecOps / product security | Integration | Machine-readable IaC inventory consumed through existing SBOM tooling, not a new tool | Reference (CycloneDX output), How-to (Dependency-Track ingestion) |
| A5 | SBOM toolchain (syft, Dependency-Track, grype) | Integration | Well-formed CycloneDX with stable component identity | Reference (schema, purl), conformance tests |
| A6 | Homelab / small-team operator | Secondary | Same output, no enterprise tooling; tolerant early adopter | Tutorial, How-to — validation channel, not the design target |
| A7 | Contributor / maintainer | Contribution | Why the two-tier model exists; why Go; why identifiers are provisional | Explanation, ADRs, C4:Component view |

## Category definitions

| Category | Focus | Roles here | Primary artifacts |
|----------|-------|-----------|-------------------|
| **Primary** | Using the tool directly | A1, A2, A3 | Tutorials, How-tos, user-facing BDD, SystemContext |
| **Integration** | Consuming the tool's output | A4, A5 | Reference docs, output-schema BDD, Container view |
| **Operational** | Running the tool in a pipeline | folded into A1/A2 — this is a CI-invoked CLI, not a service | How-tos |
| **Contribution** | Extending the tool | A7 | Explanation, ADRs, Component view |

**Note on the Operational category**: deliberately not populated with a distinct audience. The
tool is a stateless CLI invoked from CI; there is no deployment, no service to operate, and no
operator role separate from the engineers in A1/A2. Recorded rather than left blank so the gap
reads as a decision.

## Traceability

Every artifact references an audience ID:

- BDD features: `@audience:A1`
- Documentation frontmatter: `audience: A1`
- C4 persons map to Primary and Integration audiences

## Artifact coverage matrix

| Audience | BDD | Tutorial | How-to | Reference | Explanation | C4 element |
|----------|-----|----------|--------|-----------|-------------|------------|
| A1 Enterprise IaC | [ ] | [ ] | [ ] | - | - | [ ] |
| A2 OEM delivery | [ ] | - | [ ] | [ ] | - | [ ] |
| A3 Packager | [ ] | - | - | [ ] | [ ] | [ ] |
| A4 SecOps | [ ] | - | [ ] | [ ] | - | [ ] |
| A5 SBOM toolchain | [ ] | - | - | [ ] | - | [ ] |
| A6 Homelab operator | - | [ ] | - | - | - | - |
| A7 Contributor | - | - | - | - | [ ] | [ ] |

Nothing is covered yet — the project has not been bootstrapped.

---
*Created from SPARK analysis on 2026-07-30*
*Last updated: 2026-07-30*
