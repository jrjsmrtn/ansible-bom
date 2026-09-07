# requirements.yml formats

The shapes `ansible-galaxy` accepts in a requirements file, and what this tool does with each.

Written as input to the parser in [`internal/requirements`](../../internal/requirements/) and to
[#9](https://github.com/jrjsmrtn/ansible-bom/issues/9), which asks whether a declared git source
can be carried through into `lock --requirements` output.

**Anchored to two sources, and they disagree.** The structure below is the
[requirements schema bundled with ansible-lint](https://raw.githubusercontent.com/ansible/ansible-lint/main/src/ansiblelint/schemas/requirements.json)
(draft-07, read from ansible-lint **26.8.0**, 2026-09-07). The divergences are what
`ansible-galaxy` from **ansible-core 2.20.0** accepted when run against a file of that shape.
Where they disagree, the running tool wins and the divergence is recorded — this is the
[ADR-0007](../adr/0007-schema-anchor-authored-files-fixture-anchor-generated-ones.md) position:
schemas are a design reference, never a runtime validator.

⚠ **Cite the ansible-lint copy, not `ansible/schemas`.** That repository is **archived**
(2022-12-02, "Schemas are now managed inside ansible-lint project") and its requirements schema
has not changed since 2022-05-15. The two are structurally identical today — diffing them yields
only `$id` and an added `description` — so the archived copy is not *wrong*, it is merely frozen,
which is the harder kind of stale to notice.

**The three versions here move independently.** `ansible/schemas` used CalVer and stopped;
ansible-lint carries the live schema on its own release cycle; `ansible-core` is on its own again.
Nothing pins a schema version to a core version, so "the schema says X" and "core 2.20 does Y" are
observations about two unrelated artefacts and must each name what they were checked against.
Note the pair is not even self-consistent locally: ansible-lint 26.8.0 bundles ansible-core
**2.20.5**, while the `ansible-galaxy` probed above is **2.20.0**.

## Two top-level forms

A requirements file is **either** a bare sequence (legacy, roles only) **or** a mapping with
`collections` and/or `roles` keys (v2). The schema's top-level `anyOf` allows nothing else, and
the v2 form sets `additionalProperties: false` — no third section exists.

```yaml
# Legacy: a bare sequence. Every entry is a role.
- src: geerlingguy.postgresql
  version: 3.5.0

# v2: sections. At least one of collections/roles is required.
collections:
  - name: community.general
roles:
  - src: geerlingguy.postgresql
```

## Collection entries

`CollectionModel`, `additionalProperties: false`:

| Field | Notes |
|---|---|
| `name` | the identity, or a URL |
| `version` | a constraint, verbatim — no default |
| `source` | the server or path to resolve against |
| `type` | one of `galaxy`, `url`, `file`, `git`, `dir`, `subdirs` |

A collection entry may also be a **bare string** (`CollectionStringModel`), which is the common
`- community.general` form.

Note what is absent: no `src`, and no `scm`. Those are role fields.

## Role entries

`RoleModel`, `additionalProperties: false`:

| Field | Notes |
|---|---|
| `src` | the identity, a URL, or a Galaxy `namespace.name` |
| `name` | what to install it *as* — not the identity |
| `version` | **defaults to `master`** if omitted |
| `scm` | `git` or `hg`; defaults to `git` |

Roles have no `type` and no `source`. The `scm`/`type` split is the single biggest asymmetry
between the two sections.

`version` defaulting to `master` is worth dwelling on: a role entry with no version is not
unpinned in the way an unversioned collection is. It resolves to a branch name, which is mutable
and may not exist.

## Where the running tool accepts more than the schema

Both verified by running `ansible-galaxy role install -r <file>` under ansible-core 2.20.0 and
observing a successful install, not by reading documentation.

**1. A bare string is accepted in the `roles:` section.** The schema types role items as
`RoleModel` only — an object. In practice:

```yaml
roles:
  - jborean93.win_openssh    # installs; schema says this is invalid
```

**2. `include:` works, and this tool ignores it.** The legacy sequence form admits an
`IncludeModel` — `{include: <path>}` — which pulls in another requirements file. `ansible-galaxy`
follows it and installs what it finds.

```yaml
- include: more.yml
- src: geerlingguy.postgresql
  version: 3.5.0
```

`ParseBytes` returns **one** role for that file, with no error: an entry carrying neither `name`
nor `src` is skipped. Anything declared in `more.yml` is therefore invisible to `drift`, which
will report the installed roles it names as installed-but-undeclared.

This is a known gap, not a decision. It is narrow — `include:` is legacy, roles-only, and rare —
but the failure is silent, which is the part that matters.

## How this tool reads an entry

[`internal/requirements`](../../internal/requirements/) is deliberately permissive: it inventories
and compares, and does not lint. Unknown keys are tolerated rather than rejected.

- **Identity** comes from `name`, falling back to `src`. Both fields are accepted in *either*
  section, because both appear in the wild in either section, whatever the schema says.
- **`Source`** is `source` if present, else `src`.
- **`Type`** and **`SCM`** are recorded verbatim where given, and never inferred from each other.
- **A bare string** in either section becomes an entry with a name and nothing else.
- **An entry that resolves to no name at all** is skipped silently. `include:` is the case that
  reaches this path in practice.

## Git URLs and derived names

An entry whose name is a URL does not state what the content will install *as*. The last path
segment is conventionally `namespace.name`, and that is what `ansible-galaxy` uses:

```yaml
collections:
  - name: git+file:///srv/src/example.widget/    # installs as example.widget
```

`Declaration.FQN()` derives it by stripping a trailing `/`, anything after a `,`, and a `.git`
suffix, then taking the last segment. `IsDerived()` reports when that derivation was used, so a
caller can qualify what it claims rather than assert a name it inferred.

⚠ **The `,` strip is wrong here, and is [#13](https://github.com/jrjsmrtn/ansible-bom/issues/13).**
`namespace.name,version` is a **command-line** convention, not a requirements-file one. Against
ansible-core 2.20.0:

| Form | Result |
|---|---|
| `ansible-galaxy role install geerlingguy.postgresql,3.5.0` | installs 3.5.0 |
| `src: geerlingguy.postgresql,3.5.0` | downloads a role named `geerlingguy.postgresql%2C3.5` — the comma is URL-encoded, never parsed |
| `src: <git url>,3.5.0` | `fatal: repository '...,3.5.0/' not found` |
| `src: <git url>` + `version: 3.5.0` | installs (the control) |

So an entry with a comma in `src` is **broken**, and deriving a clean name from it presents a
declaration ansible cannot resolve as one this tool understands.

**The convention is not a guarantee.** A repository whose last path segment is not
`namespace.name` installs under a name this derivation gets wrong, and nothing in the file says
so.

## Bearing on #9

`lock --requirements` currently omits components it cannot express as a Galaxy `name`/`version`
pair. The format itself *can* express more than that — `type: git` with a `source`, or a role
`src` with `scm: git` — so the blocker is not the format but that the installed tree records no
trustworthy source to put there.

A requirements file does record it, because a human wrote it. Reading the declared source back is
what #9 proposes. Two things this reference makes concrete for that design:

- the field to carry is **`source` + `type`** for collections and **`src` + `scm`** for roles;
  they are not interchangeable, and a projection that writes one where the other belongs produces
  a file that fails to install
- a role's `version` defaults to `master`, so "the declaration had a version" is not the same as
  "the declaration pinned something immutable"
