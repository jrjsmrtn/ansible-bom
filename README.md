# ansible-bom

[![CI](https://github.com/jrjsmrtn/ansible-bom/actions/workflows/ci.yml/badge.svg)](https://github.com/jrjsmrtn/ansible-bom/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/jrjsmrtn/ansible-bom.svg)](https://pkg.go.dev/github.com/jrjsmrtn/ansible-bom)

**Pin what actually runs.** `ansible-bom` inventories the Ansible content installed on a control
node — collections *and* legacy roles — and produces the lockfile Ansible Galaxy never shipped, a
drift report against your `requirements.yml`, and a CycloneDX bill of materials.

> **Status: pre-release (0.x).** All four commands work, and syft catalogers exist as a separate
> module. Component identifiers are provisional until v1.0 — see
> [Design notes](#design-notes) and [Roadmap](#roadmap).

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
Ansible content at all — `syft` covers *"dozens of packaging ecosystems"* in its own words, and
Ansible is not among them.

## What it does

| Command | Purpose |
|---|---|
| `ansible-bom lock` | Emit a resolved lockfile from the installed tree — every collection and role at its exact version |
| `ansible-bom drift` | Compare installed content against `requirements.yml`: undeclared transitive content, unpinned declarations, version mismatches, mutable git sources |
| `ansible-bom scan` | Emit a CycloneDX BOM with purl identity, file hashes, licences and the dependency graph |
| `ansible-bom verify` | Check installed files against the checksums recorded at install time. Answers "has anything changed since installation?" — not "is this what upstream published?", which needs the Galaxy server or a signature |

Read-only, offline, no Galaxy credentials, no change to how you install content today.

## Installation

Download a binary for your platform from the [latest release](https://github.com/jrjsmrtn/ansible-bom/releases),
verify it, and put it on your `PATH`:

```bash
VERSION=v0.3.0          # adjust
PLATFORM=linux_amd64    # or darwin_arm64, darwin_amd64, linux_arm64, windows_amd64.exe
BASE=https://github.com/jrjsmrtn/ansible-bom/releases/download/$VERSION

curl -fsSLO $BASE/ansible-bom_${VERSION}_${PLATFORM}
curl -fsSLO $BASE/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
chmod +x ansible-bom_${VERSION}_${PLATFORM}
```

Each binary ships with a CycloneDX SBOM of its own Go dependencies alongside it.

`SHA256SUMS` is what the checksum above trusts, so from **v0.2.2** it is signed — verifying the
signature is what makes that trust worth anything. Signing is keyless, via Sigstore, and the bundle
carries the signature, the signing certificate and the Rekor inclusion proof:

```bash
curl -fsSLO $BASE/SHA256SUMS.sigstore.json
cosign verify-blob \
  --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github.com/jrjsmrtn/ansible-bom/\.github/workflows/release\.yml@refs/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS
```

Expect `Verified OK`.

The identity is the point, not the ceremony: a valid Sigstore signature only tells you *someone*
signed this. `cosign` will not let you skip it — omitting the identity flag fails with
`--certificate-identity or --certificate-identity-regexp is required for verification in keyless
mode` — and a signature made by anyone else is rejected by name:

```
expected SAN value to match regex "^https://github.com/someone-else/evil/",
got "https://github.com/jrjsmrtn/ansible-bom/.github/workflows/release.yml@refs/tags/v0.2.2"
```

Releases from v0.2.2 also carry SLSA build provenance, verifiable with
[`slsa-verifier`](https://github.com/slsa-framework/slsa-verifier). One `multiple.intoto.jsonl`
covers every binary in the release:

```bash
curl -fsSLO $BASE/multiple.intoto.jsonl
slsa-verifier verify-artifact ansible-bom_${VERSION}_${PLATFORM} \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/jrjsmrtn/ansible-bom \
  --source-tag $VERSION
```

Expect `PASSED: SLSA verification passed`.

Earlier releases (v0.1.0–v0.2.1) carry neither: they were built while the repository was private,
where keyless signing would have published the repository identity to a public transparency log.

Or build from source:

```bash
go install github.com/jrjsmrtn/ansible-bom/cmd/ansible-bom@latest
```

## Quick start

```bash
# What is actually installed, pinned
ansible-bom lock /path/to/content > ansible-bom.lock.yaml

# ...and an installable requirements.yml
ansible-bom lock --requirements /path/to/content > requirements.lock.yml

# What drifted from what you declared
ansible-bom drift -r requirements.yml /path/to/content

# A CycloneDX bill of materials
ansible-bom scan /path/to/content > bom.json

# Has anything changed since installation?
ansible-bom verify /path/to/content
```

A *content root* is a directory containing `ansible_collections/` and/or `roles/`. Pass more than
one when they live apart, which is common — `ansible.cfg` decides where they are.

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

| Milestone | Content | State |
|---|---|---|
| M1 Inventory | Collection and role parsing; `lock` | done |
| M2 Drift | `drift` against `requirements.yml` | done |
| M3 BOM | `scan` — CycloneDX output | done |
| M4 Integrity | `verify` against recorded checksums | done |
| M5 Upstream | syft catalogers | built; awaiting [anchore/syft#5129](https://github.com/anchore/syft/issues/5129) |
| **v1.0** | **Gated on the `ansible` purl type being approved and implemented** | blocked upstream |

Milestones are deliberately *not* version numbers. M1–M4 all shipped within v0.1.x, and release
versions track the published contract rather than this list.

Deferred: collection signature verification, Execution Environment images as an input, TEA
publishing. Explicitly out of scope: dependency resolution, installation, and modelling what
playbooks deploy on managed hosts.

## Contributing

Issues and pull requests are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md), which covers
the two-module layout and the one rule that matters most: the output must never claim more than it
knows.

Security reports go through [SECURITY.md](SECURITY.md), not the issue tracker — and overstated
assurance in the output counts as a security issue, not merely a bug.

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md).

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
