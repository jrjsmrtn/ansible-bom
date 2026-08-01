# Findings for purl-spec#854

Observations from implementing the proposed `ansible` purl type against real installed content,
recorded for contribution to [purl-spec#854](https://github.com/package-url/purl-spec/pull/854).

**Status: none of this has been reported upstream.** No comment from this project exists on the
PR. This file is the draft, not a record of engagement — do not cite it as though the proposal's
authors have seen it.

Each finding names the fixture or code that grounds it, so a reviewer can check the claim rather
than take it. The proposal text quoted below is the vendored snapshot at
[`internal/purl/testdata/purl-spec-854/`](../../internal/purl/testdata/purl-spec-854/); refresh it
before posting, because the wording may have moved.

Related: [ADR-0004](../adr/0004-provisional-purl-identifiers.md) records which of these we act on
and which we absorb.

---

## 1. The type is scoped to collections; roles are namespaced too, and will collide

**What the proposal says.** `type_name` is `"Ansible Collection"`, `description` is
`"Ansible collections"`, and all seven `examples` are collections. Roles are not mentioned.

**What we observe.** Galaxy namespaces roles and collections in the same `namespace.name` form and
installs them side by side on the same control node — collections under `ansible_collections/`,
roles under `roles/`. Nothing in the proposed identifier distinguishes them, so a role and a
collection sharing a namespace and name produce byte-identical purls for two different artifacts
with different content, different provenance, and different assurance
(see [ADR-0005](../adr/0005-two-tier-collection-and-role-model.md) — roles carry no checksums at
all).

Legacy roles are not a historical curiosity: `ansible-galaxy role install` is current, and mixed
trees are the norm on long-lived controllers.

**What we do about it.** Emit `?kind=role`, as a declared extension to the proposal rather than a
reading of it (`internal/purl/purl.go`, `RoleQualifier`).

**What we would ask.** Whether roles belong in this type with a discriminating qualifier, or in a
separate type. Either resolves the collision; leaving it unstated does not. The `?kind=role`
spelling is ours and we have no attachment to it — the collision is the finding.

---

## 2. `MANIFEST.json`'s `repository` field is author-supplied, and is observably wrong in the wild

**What the proposal says**, in its `note`:

> The `MANIFEST.json` file in a collection tarball contains authoritative metadata including
> namespace, name, version, and a repository field pointing to the source VCS.

**What we observe.** Namespace, name and version are authoritative — they are what Galaxy resolves
against. `repository` is not: it is copied verbatim from whatever the author left in `galaxy.yml`,
and `ansible-galaxy collection init` seeds it with a placeholder that authors frequently ship
unedited. A real published collection was found carrying the untouched skeleton value; the case is
pinned as a fixture at
[`content/testdata/collection-placeholder-repo/MANIFEST.json`](../../content/testdata/collection-placeholder-repo/MANIFEST.json),
whose `repository` is `https://www.github.com/my_org/my_collection`.

The same applies to `documentation`, `homepage`, `issues`, `authors` and `description` — all
author-supplied, none validated by Galaxy at publish time.

**What we do about it.** Parse none of them for identity, and say so at the parse site
(`content/collection.go`, the `manifest` struct comment).

**What we would ask.** That `authoritative` be narrowed to namespace, name and version, and that
the note warn against deriving `vcs_url` from `repository`. A consumer following the note as
written would emit a `vcs_url` pointing at a repository that does not exist — worse than omitting
the qualifier, because it looks resolvable.

---

## 3. Three of the four qualifiers cannot be recovered from an installed tree

The proposal already recognises half of this, in the same `note`:

> `MANIFEST.json` does not indicate which Galaxy-compatible server the tarball was downloaded
> from — this information is extrinsic and must be tracked at install time.

**What we observe.** `ansible-galaxy` does not track it at install time either, for collections.
An installed collection directory contains `MANIFEST.json` and `FILES.json` and no record of where
it came from. There is no collection equivalent of the roles' `meta/.galaxy_install_info`, and
even that file records only a version and a locale-formatted install date — not a source
(`content/role.go`, `DecodeInstallInfo`).

So for a producer scanning a control node:

| Qualifier | Recoverable from an installed tree? |
|---|---|
| `repository_url` | **No** — not recorded anywhere on disk |
| `vcs_url` | **No** — and `repository` is not a substitute (finding 2) |
| `download_url` | **No** — not recorded anywhere on disk |
| `packaging` | **Partially** — via the OS package database, as the proposal itself describes |

Four of the seven published examples carry a qualifier in the "No" rows. That is fine for a
producer that *performs* the install and can observe the source, and impossible for one that
inspects the result afterwards — which is what an SBOM generator running on a control node does.

**What we do about it.** Emit none of them. Git-sourced content is reported through the drift
channel instead, where a mutable source is a finding rather than an identifier detail.

**What we would ask.** That the definition state which qualifiers are available to an
install-time producer versus a filesystem scanner. Without it, two conformant tools scanning the
same host emit different identifiers for the same artifact, and the type's join key stops being
one. This bears directly on the open `vcs_url` review thread: the syntax disagreement is moot for
any consumer that cannot obtain the value.

---

## Before posting

> **Currency check, 2026-08-02.** purl-spec#854 is still **open and unmerged**, last updated
> 2026-06-09 — so the vendored snapshot and the quotations below are current, and the divergence
> the conformance test pins is unchanged. The `ansible` type is still absent from the registry:
> 42 type definitions exist upstream and none of them is `ansible`.

1. Refresh the vendored snapshot (see
   [`PROVENANCE.md`](../../internal/purl/testdata/purl-spec-854/PROVENANCE.md)) and re-read the
   proposal — it has changes requested, and this file is written against a snapshot taken
   2026-07-31.
2. Re-check whether the review threads already cover any of this.
3. Post findings separately. Finding 1 is a scope question for the type's authors; findings 2 and
   3 are corrections to the `note` and are independently actionable.
