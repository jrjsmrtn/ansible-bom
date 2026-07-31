# 4. Provisional `pkg:ansible` Identifiers, and a Gated 1.0

Date: 2026-07-31

## Status

Accepted

## Context

Component identity is the load-bearing part of any BOM. A BOM whose identifiers do not match what
consumers expect is worse than no BOM: it looks authoritative and joins to nothing.

The identifier standard is [purl](https://github.com/package-url/purl-spec) (ECMA-427). **There is
no registered `ansible` type.** [PR #854](https://github.com/package-url/purl-spec/pull/854),
opened 2026-04-06, proposes one as a machine-readable type definition
(`types/ansible-definition.json`, conforming to `purl-type-definition.schema-1.0.json`): namespace
and name as **separate, both-required** components, an optional version, and optional
`repository_url`, `vcs_url`, `download_url` and `packaging` qualifiers. Its first example is
`pkg:ansible/cisco/aci@2.13.0`. At the time of writing it has one approval and one
changes-requested, with live disagreement over whether `vcs_url` should use Ansible's
comma-before-version syntax and whether `packaging=rpm|deb` should defer to the existing rpm/deb
types.

We cannot wait for it — the lockfile and drift capabilities need none of this and deliver value
immediately. We also cannot pretend it is settled.

## Decision

### Emit a provisional form, and pin how it diverges

0.x emits `pkg:ansible/<namespace>.<name>@<version>` — the Ansible-native fully-qualified name in a
single segment.

**This does not conform to PR #854, and an earlier revision of this ADR wrongly claimed it did.**
The proposal makes `namespace` a *required, separate* purl component, so what this tool emits
carries no namespace at all: `pkg:ansible/cisco.aci@2.13.0` against the proposal's
`pkg:ansible/cisco/aci@2.13.0`. That is non-conformance with a machine-readable constraint, not a
variant spelling, and the eventual migration is therefore a real change rather than the no-op this
ADR previously predicted.

The error survived for months because the proposal was **paraphrased into prose here and never
captured**. Nothing could contradict the paraphrase. The correction is not just the sentence:

- The proposal is **vendored** at `internal/purl/testdata/purl-spec-854/`, with its provenance,
  upstream commit and digest recorded alongside it, clearly marked as an unmerged proposal.
- `internal/purl/conformance_test.go` asserts this tool's output against that file and **pins the
  divergence**, so it fails if either side moves — the tool made to conform, or the proposal
  amended. Both failure directions were exercised before the tests were committed.

The divergence stands for now rather than being fixed immediately: `?kind=role` is also outside the
proposal, roles are outside its scope entirely, and changing identity is worth doing **once**, when
the shape is settled, not twice. Construction remains in one function so that it is one edit.

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
- The divergence from the proposal is now pinned by a test rather than asserted in prose, so
  neither side can move unnoticed.

**Negative**:

- **1.0 depends on something outside our control** and could be a long way out. PR #854 has
  visible disagreement, and "implemented" is a further step beyond "approved". The project may sit
  at 0.x for a long time, which some adopters read as immaturity regardless of quality.
- BOMs emitted during 0.x may need regenerating. Acceptable: BOMs are cheap to re-emit, which is
  why this is a regeneration path and not a migration path.
- If PR #854 is rejected outright rather than amended, the fallback is unclear and this ADR is
  superseded.

**Risks**:

- *Divergence.* **Already realised**, in exactly the namespace/name handling this risk named: BOMs
  emitted by 0.x are wrong against the proposal, not merely provisional, and will need re-emitting.
  The output marker and the 0.x version are what make that acceptable. The mitigation this entry
  originally gave — "track the PR" — had no failure mode and did not work; it is replaced by the
  vendored snapshot and conformance test described above.
- *Adopter confusion.* Someone may pin a 0.x identifier into their own tooling. The output marker
  is the mitigation; the README states it too.

## References

- [purl-spec PR #854](https://github.com/package-url/purl-spec/pull/854) — the proposal being tracked
- `internal/purl/testdata/purl-spec-854/` — vendored snapshot of the proposal and its provenance
- `internal/purl/conformance_test.go` — the divergence, pinned
- [SPARK analysis](../inception/spark-analysis.md) — risk R4 and the release-gating section
- [ECMA-427](https://www.ecma-international.org/publications-and-standards/standards/ecma-427/)
