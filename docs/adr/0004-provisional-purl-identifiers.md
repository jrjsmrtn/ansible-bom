# 4. Provisional `pkg:ansible` Identifiers, and a Gated 1.0

Date: 2026-07-31

## Status

Accepted

## Context

Component identity is the load-bearing part of any BOM. A BOM whose identifiers do not match what
consumers expect is worse than no BOM: it looks authoritative and joins to nothing.

The identifier standard is [purl](https://github.com/package-url/purl-spec) (ECMA-427). **There is
no registered `ansible` type.** [PR #854](https://github.com/package-url/purl-spec/pull/854),
opened 2026-04-06, proposes one — namespace and name from the collection identity
(`community.general`), version from the collection version, with `repository_url`, `vcs_url`,
`download_url` and `packaging` qualifiers. At the time of writing it has one approval and one
changes-requested, with live disagreement over whether `vcs_url` should use Ansible's
comma-before-version syntax and whether `packaging=rpm|deb` should defer to the existing rpm/deb
types.

We cannot wait for it — the lockfile and drift capabilities need none of this and deliver value
immediately. We also cannot pretend it is settled.

## Decision

### Emit the proposed form, provisionally

0.x emits `pkg:ansible/<namespace>.<name>@<version>` following PR #854's proposal.

- **Construction lives in exactly one function.** The eventual change must be one edit.
- **Avoid `vcs_url`.** It is the actively contested qualifier; git-sourced content is instead
  reported through the drift channel, where mutable sources are a finding rather than an
  identifier detail.
- **Label the output.** BOMs emitted by 0.x carry an explicit marker that identifiers are
  provisional and subject to change. A consumer must not discover this by having a join silently
  break.

### Gate 1.0 on the type being approved *and* implemented

**1.0 is not a feature milestone.** It is released when the `ansible` purl type is:

1. **approved** — merged into the purl specification, and
2. **implemented** — recognised by purl libraries and the BOM consumers that matter
   (Dependency-Track, syft, grype).

Approval alone is insufficient. An identifier no tool recognises produces the same broken joins as
an unregistered one.

Version numbers below 1.0 signal exactly what is true: the tool works, and its identity contract
is not yet stable.

### Do not invent a fallback type

Rejected: emitting `pkg:generic/...` or a bespoke type in the meantime. It would create a second
identifier scheme to migrate away from, and `generic` purls join to nothing anyway — the cost of
being wrong in the proposed direction is lower than the cost of being deliberately non-standard.

## Consequences

**Positive**:

- 0.x ships now; the value that needs no standard is not held hostage to one.
- One-edit migration when the type settles.
- Honest versioning — 1.0 means the identity contract is stable, not that a backlog is empty.
- Choosing the proposed form makes the eventual change most likely to be a no-op.

**Negative**:

- **1.0 depends on something outside our control** and could be a long way out. PR #854 has
  visible disagreement, and "implemented" is a further step beyond "approved". The project may sit
  at 0.x for a long time, which some adopters read as immaturity regardless of quality.
- BOMs emitted during 0.x may need regenerating. Acceptable: BOMs are cheap to re-emit, which is
  why this is a regeneration path and not a migration path.
- If PR #854 is rejected outright rather than amended, the fallback is unclear and this ADR is
  superseded.

**Risks**:

- *Divergence.* If the merged type differs from the proposal in namespace/name handling — not just
  in qualifiers — previously emitted BOMs become wrong rather than merely provisional. Mitigate by
  tracking the PR and re-emitting fixtures on any change.
- *Adopter confusion.* Someone may pin a 0.x identifier into their own tooling. The output marker
  is the mitigation; the README states it too.

## References

- [purl-spec PR #854](https://github.com/package-url/purl-spec/pull/854) — the proposal being tracked
- [SPARK analysis](../inception/spark-analysis.md) — risk R4 and the release-gating section
- [ECMA-427](https://www.ecma-international.org/publications-and-standards/standards/ecma-427/)
