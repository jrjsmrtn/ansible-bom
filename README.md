# ansible-bom

**Pin what actually runs.** `ansible-bom` inventories the Ansible content installed on a control
node — collections *and* legacy roles — and produces the lockfile Ansible Galaxy never shipped, a
drift report against your `requirements.yml`, and a CycloneDX bill of materials.

> **Status: pre-release (0.x).** Not yet usable. This repository currently contains the design
> analysis and decision records. See [Roadmap](#roadmap).

---

## Why

Ansible's central promise is **idempotent convergence**: run the playbook again, get the same
state. That promise quietly assumes the modules themselves are unchanged.

With unpinned collections and roles — the default, and near-universal in practice — a run in
January and a run in June execute a byte-identical playbook through *different code*. Module
defaults shift, behaviours are deprecated, bugs are fixed and introduced. The playbook is the
same; the convergence is not. Nothing in the ecosystem reports this.

The consequences are concrete:

- **Check mode is untrustworthy** — a dry run predicts what *today's* module would do.
- **Host drift is ambiguous** — you cannot separate configuration drift from tooling drift.
- **Forensics are impossible** — "what code ran on that host in March?" has no answer.

Ansible Galaxy has no lockfile. The feature was requested against trackers now archived, built
once in `mazer`, and abandoned with it in 2020. Meanwhile no mainstream SBOM generator catalogs
Ansible content at all — `syft` covers 61 ecosystems; Ansible is not among them.

## What it will do

| Command | Purpose |
|---|---|
| `ansible-bom lock` | Emit a resolved lockfile from the installed tree — every collection and role at its exact version |
| `ansible-bom drift` | Compare installed content against `requirements.yml`: undeclared transitive content, unpinned declarations, version mismatches, mutable git sources |
| `ansible-bom scan` | Emit a CycloneDX BOM with purl identity, file hashes, licences and the dependency graph |
| `ansible-bom verify` | Check installed files against the checksums recorded at install time |

Read-only, offline, no Galaxy credentials, no change to how you install content today.

## Who it is for

- **Platform / IaC engineers** — reproducible control planes, and idempotency you can rely on
  across time
- **OEM delivery teams** — xBOMs for delivered infrastructure, where Ansible content is currently
  a hole in the provenance chain
- **Packagers** — authoritative component lists for redistributed content
- **SecOps** — machine-readable IaC inventory, consumed through the SBOM tooling you already run

## Design notes

Two facts shape the tool and are worth knowing before you file an issue:

**Roles and collections are not equivalent.** Collections ship `MANIFEST.json` and `FILES.json`
with a sha256 for every file. Roles ship neither — a Galaxy-installed role's only provenance is a
version string in `meta/.galaxy_install_info`. `ansible-bom` reports both, and is explicit about
which tier a component is in rather than emitting entries that falsely look equivalent.

**Identifiers are provisional until v1.0.** There is no registered `ansible` purl type yet;
[purl-spec#854](https://github.com/package-url/purl-spec/pull/854) proposes one and is still open.
0.x emits the proposed form, clearly labelled. **v1.0 waits on that type being approved *and*
implemented** — 1.0 is a compatibility promise, and it would be dishonest to make it over
identifiers that may still change.

## Roadmap

| Milestone | Content |
|---|---|
| 0.1 | Collection + role inventory; `lock` |
| 0.2 | `drift` against `requirements.yml` |
| 0.3 | `scan` — CycloneDX output |
| 0.4 | `verify`; syft cataloger |
| 1.0 | Gated on the `ansible` purl type being approved and implemented |

Deferred: collection signature verification, Execution Environment images as an input, TEA
publishing. Explicitly out of scope: dependency resolution, installation, and modelling what
playbooks deploy on managed hosts.

## Documentation

- [SPARK analysis](docs/inception/spark-analysis.md) — the design document: problem, evidence,
  alternatives, risks
- [Audience registry](docs/reference/audience-registry.md)
- [Architecture Decision Records](docs/adr/)

## Licence

[Apache-2.0](LICENSE).

## Trademark

Not affiliated with, endorsed by, or sponsored by Red Hat, Inc. "Ansible" and "Ansible Galaxy" are
trademarks of Red Hat, Inc. This project uses the name descriptively, to identify the ecosystem it
operates on.
