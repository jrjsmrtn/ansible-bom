# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Versioning note**: 0.x carries provisional component identifiers. **1.0 is gated on the
`ansible` purl type being approved and implemented upstream** — see
[ADR-0004](docs/adr/0004-provisional-purl-identifiers.md).

## [Unreleased]

### Added

- Release workflow: cross-compiled binaries for linux/darwin/windows on amd64 and arm64, a
  `SHA256SUMS` file, and a CycloneDX SBOM **per binary**, catalogued from the built artefact so it
  records the linked Go module graph and `pkg:golang/stdlib`, with a CI assertion that fails the
  release if the scan ever catalogues the wrong subject
- SLSA build provenance and cosign signing of the checksum file, both gated on the repository
  being public — keyless signing publishes the repository identity to a public transparency log
- Version is injected from the git tag at build time and defaults to `dev`, so an untagged local
  build cannot claim to be a release

## [0.1.0] - 2026-07-31

First release. All four commands work; identifiers are provisional until v1.0.

### Added

- SPARK inception analysis, measured against a real Ansible control node
- Audience registry covering seven audiences across four Diátaxis categories
- Foundation ADRs (0001–0003) and decision ADRs (0004–0007)
- POC-2: confirmed OSV has no Ansible ecosystem, and that the gap surfaces as zero findings
  rather than as an error
- Go module scaffold, Apache-2.0 licence, pre-commit quality gate
- `internal/content`: collection and role parsers, and a tree scanner
  - collections parsed from `MANIFEST.json` + `FILES.json`, with `format` as a hard version gate
  - roles parsed from `meta/main.yml` and `meta/.galaxy_install_info`, always name-and-version-only
  - dependency constraints recorded verbatim, never resolved
  - `.info` marker directories ignored; unparseable content reported rather than dropped
- Parser fixtures captured from real installed content, with provenance recorded in
  `internal/content/testdata/README.md`
- `ANSIBLE_BOM_REAL_TREE` env-guarded test for running the parser against a real control node
- `internal/lockfile`: lockfile rendering and an installable `requirements.yml` projection
  - unversioned content is recorded as `unpinnable`, never dropped and never given an invented
    version — a lockfile that omits it would overstate the reproducibility it provides
  - per-collection content digest over the file manifest, so a version republished with different
    content is detectable; roles get none, because nothing exists to derive one from
  - the header states what the file does not assert, where a reader will see it
- `ansible-bom lock` command, with `--output`, `--requirements` and `--fail-on-problems`
- collection origin inferred from `.info` install markers: presence means Galaxy, absence means
  unknown rather than git
- `internal/requirements`: permissive `requirements.yml` parser — modern and legacy forms, bare
  strings, `name`/`src` spellings, git URLs with ansible's comma-separated ref
- `internal/drift`: comparison of installed content against declarations
  - findings ordered worst-first: mutable source, unpinnable, version mismatch, undeclared,
    missing, unpinned
  - first-party roles reported as context rather than as drift
- `ansible-bom drift` command, with `--requirements` and `--fail-on-drift`
- `internal/purl`: the single construction site for provisional `pkg:ansible` identifiers, with a
  `kind=role` qualifier because the upstream proposal addresses collections only
- `internal/cyclonedx`: CycloneDX 1.6 emission
  - per-component assurance tier and vulnerability-coverage status, with the reason stated
  - content digest carried as a property, never as `hashes` — it is computed over the file
    manifest, not over a distributed artefact
  - dependency graph linking only components present in the BOM
  - `compositions` declaring the inventory incomplete whenever content could not be parsed
- `ansible-bom scan` command, with `--output` and `--fail-on-problems`
- `scripts/validate-bom.py`: schema conformance check against the official CycloneDX schema
- `internal/verify`: checks installed files against the checksums recorded at install time
  - verifies `FILES.json` against the digest `MANIFEST.json` records for it *before* trusting the
    per-file checksums it contains
  - roles are reported as unverifiable and never as verified
  - verified and unverifiable counts are kept separate so one cannot be read as the other
- `ansible-bom verify` command, with `--quiet` and `--exit-zero`; exits non-zero on failure by
  default, unlike the other commands
- CLI end-to-end tests covering all four commands, their flags and their exit codes
- exit status 2 for a malformed invocation, distinct from 1 for a failed operation
- GitHub Actions CI: build, gofmt, vet, race-enabled tests with coverage, govulncheck,
  CycloneDX schema conformance, and dependency review on PRs
- OpenSSF Scorecard workflow plus a weekly scheduled govulncheck
- Dependabot for Go modules and GitHub Actions, with a 7-day cooldown
- all actions SHA-pinned; read-only default token permissions; no credentials persisted
- `post-merge` hook (`scripts/sync-remotes.sh`) propagating a merged branch to every remote that
  already tracks it, so a pull request merged on one forge does not leave the other silently
  behind
- Upstream cataloger proposal opened at anchore/syft#5129
- Confirmed from `ansible-core` 2.20.0 source that `ansible-galaxy` has no extension point,
  upgrading a negative-evidence finding to a verified one

[Unreleased]: https://github.com/jrjsmrtn/ansible-bom/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jrjsmrtn/ansible-bom/releases/tag/v0.1.0
