# 7. Schema-Anchor Authored Files, Fixture-Anchor Generated Ones

Date: 2026-07-31

## Status

Accepted

## Context

`ansible-bom` reads five file formats. They divide sharply by whether anyone has written down what
they contain.

**Authored files — a published JSON Schema exists.** The [`ansible/schemas`](https://github.com/ansible/schemas)
repository, consumed by ansible-lint and SchemaStore, publishes schemas for the files a human
writes:

| File | Schema | Notes |
|---|---|---|
| `requirements.yml` | `ansible-requirements.json` | |
| `meta/main.yml` | `ansible-meta.json` | titled "Meta Schema **v1/v2**" — two shapes, not one |
| `galaxy.yml` | `ansible-galaxy.json` | source of the fields that end up in `MANIFEST.json` |

**Generated files — nothing.** `MANIFEST.json` and `FILES.json` have no schema anywhere.
`ansible-core` ships no schema files at all; the structure exists as a Python dict literal in
`ansible/galaxy/collection/__init__.py:_build_manifest()`, with key sets pinned only as `t.Literal`
type aliases (`ManifestKeysType`, `FileMetaKeysType`) that serve the type checker rather than any
validator. The `format: 1` field marks a version that nothing formally describes.

The asymmetry is unfortunate in exactly the wrong direction: **the two files carrying all the
integrity data are the two with no specification.** Every sha256 this tool reports comes from a
format defined only by the code that writes it.

A related discovery: `ansible-meta.json` covering "v1/v2" means role metadata has at least two
shapes in the wild. That was not in the inception analysis.

## Decision

Anchor each format to the strongest artefact available for it, and do not pretend the anchors are
equivalent.

### Authored files — schema as design reference and test oracle

`requirements.yml` and `meta/main.yml` parsers are written against the published schemas rather
than against guesses or a single observed example. Test cases are derived from the schemas'
structure — including both role-meta shapes, which a sample of real trees might not contain.

### Generated files — the parser is the contract, fixtures are the defence

For `MANIFEST.json` and `FILES.json` there is nothing to conform to. Therefore:

- **Captured fixtures from real content trees are load-bearing**, not a convenience. They are the
  closest thing to a specification that exists, which promotes ADR-0002's fixture practice from
  good hygiene to the only defence against silent drift.
- **`format` is a version gate.** On an unrecognised `format` value the tool reports the component
  as unparseable and says why. It does not guess. Mis-parsing a checksum manifest is worse than
  declining to parse it.
- **Only namespace, name and version are trusted** (already ADR-0002 §5); the rest is
  informational, which limits the blast radius of an undetected format change.

### Do not validate at runtime

The schemas are **not** enforced against user files at run time. This tool inventories what is
present; it is not a linter. A `requirements.yml` with an unknown key, or a role meta that fails
strict validation, must still be inventoried — refusing would make the tool useless precisely in
the messy estates that need it most. `ansible-lint` already occupies the validation role.

Schemas are therefore consulted at development time, not shipped as a runtime dependency.

**Alternatives rejected**:

| Alternative | Why not |
|---|---|
| Validate authored files against the schemas at runtime | Wrong posture — we inventory, we do not police. Would fail on real-world files that Ansible itself accepts, and adds a validator dependency |
| Ignore the schemas entirely; parse from observation | Throws away a free oracle, and would have missed the v1/v2 role-meta split that no single sampled tree revealed |
| Generate Go structs from the schemas | Codegen for three small files is disproportionate, and it does nothing for the two formats that matter most, which have no schema |
| Vendor schema copies into the repo | Staleness with no upstream signal; the schemas are a development reference, and pinning a stale copy is worse than reading the current one |

## Consequences

**Positive**:

- Parsers for authored files are anchored to a real specification rather than to one sampled tree.
- The v1/v2 role-meta split is handled deliberately instead of being discovered as a bug report.
- The `format` gate converts an undetectable failure (silently mis-parsed checksums) into a
  visible one.
- No runtime schema dependency, and no risk of rejecting files Ansible itself accepts.

**Negative**:

- Fixture capture and curation become a maintenance obligation with no upstream to inherit from.
- Nothing warns us when `MANIFEST.json` or `FILES.json` changes shape; we find out when a real
  tree fails to parse. The `format` gate limits the damage but does not provide notice.
- The published schemas can themselves drift or be wrong; they describe intent, not what
  `ansible-core` actually accepts.

**Risks**:

- *False confidence in the schema half.* A schema-anchored parser can still be wrong if the schema
  and the runtime disagree. Where they conflict, `ansible-core`'s behaviour wins and the fixture
  is updated to record the divergence.

## References

- [ansible/schemas](https://github.com/ansible/schemas) — the published JSON Schemas
- `ansible/galaxy/collection/__init__.py` — `_build_manifest()`, `ManifestKeysType`,
  `FileMetaKeysType` (verified against `ansible-core` 2.20.0, 2026-07-31)
- [ADR-0002](0002-adopt-development-best-practices.md) §1, §5 — fixture practice and trusted fields
- [ADR-0005](0005-two-tier-collection-and-role-model.md) — the parallel asymmetry in assurance
