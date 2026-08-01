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

### Conform to the proposal, and track it until the type is registered

0.x emits `pkg:ansible/<namespace>/<name>@<version>` — the shape PR #854 defines, with `namespace`
as a **separate component**, both components lowercased.

The alternative — keeping Ansible's native `namespace.name` in a single segment — was rejected. If
the aim is to conform, there is no value in a third syntax that matches neither the proposal nor
anything else. An identifier that is deliberately different is not more honest than one that is
provisionally aligned; it is just harder to migrate and impossible to join.

**An earlier revision of this ADR claimed conformance while emitting the dotted form.** That was
wrong, and it survived for months because the proposal was paraphrased into prose here and never
captured — nothing could contradict the paraphrase. The correction is structural, not editorial:

- The proposal is **vendored** at `internal/purl/testdata/purl-spec-854/`, with its upstream
  commit, digest and PR state recorded, clearly marked as an unmerged proposal.
- `internal/purl/conformance_test.go` round-trips **every example the proposal publishes** through
  this tool's construction and requires the same identifier back. The expectations come from
  upstream, so refreshing the snapshot re-tests conformance instead of re-asserting a stale reading
  of it.

Conformance is to a **moving target** and must be re-established, never assumed: the type is not
registered, and #854 is open with changes requested on `vcs_url` syntax and the `packaging`
qualifier. The tests fail if either side moves, which is what makes "track the proposal" a
commitment rather than an intention.

**Two departures, both deliberate and both asserted as such:**

- **`?kind=role` is kept.** The proposal is scoped to collections and has no equivalent. Galaxy
  namespaces roles and collections alike, so `author/name@version` is ambiguous between them and
  identifiers would silently collide without a discriminator. Carrying a qualifier the proposal has
  not considered is better than emitting colliding identity. **A separate proposal covering roles
  is intended**; until then this is an extension, and the code and tests say so.
- **No namespace is emitted when none was observed.** The proposal marks it required because it
  models Galaxy-installed collections, which always have one; a locally-authored role under
  `roles/` genuinely has none. Such a purl is knowingly incomplete. Inventing a namespace to
  satisfy a schema would fabricate identity, which this tool must never do.

Also unchanged: construction stays in **one function**, `vcs_url` is still avoided as the contested
qualifier, and 0.x output still carries the marker saying identifiers are provisional.

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
- Conformance is pinned by tests driven by the proposal's own examples, rather than asserted in
  prose, so neither side can move unnoticed.
- Migration when the type is registered is now most likely a no-op for collections, which is what
  choosing the proposed form was always supposed to buy.

**Negative**:

- **1.0 depends on something outside our control** and could be a long way out. PR #854 has
  visible disagreement, and "implemented" is a further step beyond "approved". The project may sit
  at 0.x for a long time, which some adopters read as immaturity regardless of quality.
- BOMs emitted during 0.x may need regenerating. Acceptable: BOMs are cheap to re-emit, which is
  why this is a regeneration path and not a migration path.
- If PR #854 is rejected outright rather than amended, the fallback is unclear and this ADR is
  superseded.

**Risks**:

- *Divergence.* Realised once already, in exactly the namespace/name handling this risk named, and
  undetected for months. **Identifiers emitted before v0.3.0 are wrong** — `pkg:ansible/cisco.aci`
  rather than `pkg:ansible/cisco/aci` — and BOMs from those releases need re-emitting rather than
  merely re-labelling. The mitigation this entry originally gave, "track the PR", had no failure
  mode and did not work; it is replaced by the vendored snapshot and conformance tests.
- *The proposal is amended before it merges.* Likely, given two open review threads. Re-emission is
  cheap and the tests make the change visible; this is the accepted cost of tracking rather than
  waiting.
- *Adopter confusion.* Someone may pin a 0.x identifier into their own tooling. The output marker
  is the mitigation; the README states it too.

## References

- [purl-spec PR #854](https://github.com/package-url/purl-spec/pull/854) — the proposal being tracked
- `internal/purl/testdata/purl-spec-854/` — vendored snapshot of the proposal and its provenance
- `internal/purl/conformance_test.go` — the divergence, pinned
- [SPARK analysis](../inception/spark-analysis.md) — risk R4 and the release-gating section
- [ECMA-427](https://www.ecma-international.org/publications-and-standards/standards/ecma-427/)
