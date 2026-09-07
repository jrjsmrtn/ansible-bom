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
| `version` | omitted is `""` at parse time; see the note below on `master` |
| `scm` | `git` or `hg`; defaults to `git` |

Roles have no `type` and no `source`. The `scm`/`type` split is the single biggest asymmetry
between the two sections.

**The `master` default is a runtime fallback, not a parse-time one.** The schema and the CLI
docstring both say `version` defaults to `master`, and that is misleading:
`RoleRequirement.role_yaml_parse` sets `role['version'] = ''` when the key is absent
(`playbook/role/requirement.py:110`). `master` appears later, in `galaxy/role.py:284`, and only
when the Galaxy API returns no versions for the role **and** no `github_branch`. So an omitted
version resolves to the newest version the API lists, or the project's default branch, or
literally `master` — three different outcomes the file cannot distinguish.

Either way the point stands: a role entry with no version is not unpinned the way an unversioned
collection is. It resolves to something chosen at install time, which is mutable and may not
exist.

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

## Where the parsing actually happens

The schema describes the shape; these three functions decide the meaning. Read them before
changing anything here — the file's semantics are not derivable from its structure.

| Function | Location (ansible-core 2.20.0) | Does |
|---|---|---|
| `GalaxyCLI._parse_requirements_file` | `cli/galaxy.py:732` | picks v1/v2, rejects unknown top-level keys, routes each entry |
| `RoleRequirement.role_yaml_parse` | `playbook/role/requirement.py:65` | normalises a role entry; the comma split lives here |
| `RoleRequirement.repo_url_to_role_name` | `playbook/role/requirement.py:49` | derives the installed name from a URL |

Three behaviours worth knowing, none of them in the schema:

- **The comma separator applies to bare-string roles only.** `- ns.name,3.5.0` splits into
  `src` + `version` (and a third field is the install-as name; more than two commas is an error).
  The mapping `src:` path does **not** split, despite a stale comment in ansible reading
  `# New style: { src: 'galaxy.role,version,name' }`.
- **`repo_url_to_role_name` strips in a fixed order**: last `/` segment, then `.git`, then
  `.tar.gz`, then everything after a `,`. Order matters — `…/r.git,v1.2.3` yields **`r.git`**,
  because the `.git` no longer sits at the end when that check runs. It does not trim a trailing
  slash, so `http://x/repo/` derives an **empty** name.
- **Strictness is inverted between levels.** An unknown key on a role entry is silently dropped
  (`VALID_SPEC_KEYS` is `name`, `role`, `scm`, `src`, `version`); an unknown key at the top level
  is a hard error, as is an empty file.

⚠ `Declaration.FQN()` **approximates** `repo_url_to_role_name` and diverges from it in four ways —
strip order, `.tar.gz`, trailing slash, and URL detection. Tracked in
[#13](https://github.com/jrjsmrtn/ansible-bom/issues/13).

## Recovering a declared source (implemented, #9)

`lock --requirements` omits components it cannot express as a Galaxy `name`/`version` pair,
because the installed tree records no trustworthy source. Pass `-r <requirements.yml>` and the
source is recovered from the file the operator wrote, and emitted **verbatim**:

```console
$ ansible-bom lock --requirements -r requirements.yml ./content
collections:
  - name: community.general
    version: 11.4.0
  - name: community.windows
    version: main
    type: git
    source: https://github.com/example/community.windows.git
```

Three constraints follow from the sections above, and they are the reason this is not simply "copy
the file across":

- **The field differs by section.** Collections carry `source` + `type`; roles carry `src` +
  `scm`. Writing one where the other belongs produces a file that fails to install.
- **Only declarations this tool can match by name are usable.** A collection declared by URL has
  no name until its artefact is fetched, so it cannot be tied to an installed component. It stays
  omitted, with its declared source named in the omission list.
- **A carried ref is usually not a pin.** Only a full 40-character commit SHA is treated as
  immutable; a tag can be moved or deleted upstream. Anything else is listed under `CARRIED
  THROUGH … NOT immutably pinned`, in the file and on stderr, because a reinstall follows the ref
  rather than reproducing this tree.

Nothing is emitted for a component with no matching declaration: `-r` recovers sources, it never
invents them.
