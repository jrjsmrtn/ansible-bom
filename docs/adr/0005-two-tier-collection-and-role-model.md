# 5. Model Collections and Roles as Two Tiers of Assurance

Date: 2026-07-31

## Status

Accepted

## Context

Roles are first-class in this tool: real control nodes run both, and a BOM that covered only
collections would be silently incomplete for most users.

But the two are not equivalent, and the gap is in exactly the property a BOM exists to convey.
Measured on a real control node during the [SPARK analysis](../inception/spark-analysis.md):

| | Collections | Roles |
|---|---|---|
| Manifest | `MANIFEST.json` — namespace, name, version, dependencies, licence | `meta/main.yml` — no version |
| Install provenance | `MANIFEST.json` + `<ns>.<name>-<version>.info` marker directory | `meta/.galaxy_install_info` — version and a **locale-formatted** date; **absent entirely** for locally-authored roles |
| File checksums | **sha256 on every regular file** (3290/3290 in one 4318-entry manifest) | **None. No equivalent file exists.** |
| Uniform across install sources? | Yes — `ansible-galaxy` builds the artifact from a git checkout before installing | No |

A role's entire recorded identity is a version string written by the installer into a dotfile. It
can be edited by anyone with write access to the tree, and there is nothing to verify it against.

The temptation is to emit both as ordinary CycloneDX components and let the absence of hashes
speak for itself. It does not: a consumer reading a component list has no way to distinguish "this
component has no hashes because the tool did not collect them" from "this component has no hashes
because none exist anywhere".

## Decision

Model assurance explicitly as **two tiers**, and make the tier visible in every output.

**Tier 1 — collections.** Full identity, dependency graph, per-file sha256. Verifiable against
recorded checksums.

**Tier 2 — roles.** Name and version only. No integrity data. Version provenance is
`meta/.galaxy_install_info` where present; locally-authored roles have no version at all and are
reported as unversioned rather than assigned a placeholder.

Consequences for each output:

- **BOM** — every component carries a property recording its assurance tier and the reason
  (`checksums-unavailable-for-roles`). Absence of hashes is stated, not implied.
- **`verify`** — verifies collections; reports roles as **unverifiable**, never as "passed".
  Reporting an unverifiable component as verified would be the worst failure this tool could have.
- **`lock`** — locks both. A role version pin is still worth having for idempotency even without
  integrity data; that is the lockfile's purpose, distinct from the BOM's.
- **Summary output** — states counts per tier, so "42 components inventoried" cannot be
  misread as "42 components verified".

**Do not paper over the gap.** Rejected alternatives: computing our own hashes over the role
directory at scan time (they attest to nothing — there is no reference to compare against, and it
would manufacture false confidence); omitting roles from the BOM entirely (incomplete inventory,
which is the failure mode SBOMs exist to prevent); marking roles as `unknown` completeness at the
document level (too coarse — it taints the collection data too).

## Consequences

**Positive**:

- The BOM never overstates its own assurance, which is the single most damaging thing an SBOM can
  do.
- Roles are inventoried and pinnable despite carrying no integrity data.
- The distinction is machine-readable, so consumers can filter or weight by tier.

**Negative**:

- Non-standard properties: CycloneDX has no first-class notion of "assurance tier", so this rides
  on component `properties` and needs documenting in the output reference for consumers to use it.
- More output surface to explain, and a summary that is less satisfying than a single number.
- If Ansible ever adds checksums to roles, the tier model becomes vestigial — a good problem,
  handled by superseding this ADR.

**Risks**:

- *Consumers ignore properties.* Most tooling will surface roles as ordinary components regardless
  of what we annotate. Mitigate by also stating the split in human-readable output, where a person
  will actually see it.

## References

- [SPARK analysis](../inception/spark-analysis.md) — the on-disk measurements above, and risk R2
- [ADR-0004](0004-provisional-purl-identifiers.md) — identity for both tiers is provisional until 1.0
