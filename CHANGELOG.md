# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Versioning note**: 0.x carries provisional component identifiers. **1.0 is gated on the
`ansible` purl type being approved and implemented upstream** — see
[ADR-0004](docs/adr/0004-provisional-purl-identifiers.md).

## [Unreleased]

### Added

- Community health files for public release: `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`
  (Contributor Covenant 2.1), `SECURITY.md`, issue and pull-request templates, and
  `.github/release.yml`
- SPDX headers on all 25 Go source files
- CodeQL analysis workflow
- README install instructions, quick start, badges and contribution pointers

### Changed

- ADR-0008 corrected: the cataloger module is not independently consumable because the repository
  is **private**, not because no tag contained `content/`. Tagging v0.2.0 and dropping the
  `replace` was tried and fails — the module proxy and checksum database cannot read a private
  repository, and a `go.work` workspace does not avoid the verification. The exit is the
  `public-release` gate

## [0.2.0] - 2026-07-31

Publishes the parsers as a public API and adds syft catalogers as a separate module. No change to
the CLI's behaviour or to its dependency graph, which remains three modules.

### Added

- `cataloger/`: syft catalogers for Ansible collections and legacy roles, as a **separate Go
  module** so the shipped CLI never acquires syft's dependency graph — measured at 3 modules
  versus 445 (ADR-0008). Tested through syft's real directory resolver
- CI asserts the root module remains free of syft
- Direct tests for the reader-based decoders, which are the public contract this release
  publishes and were previously exercised only through the path-based wrappers

### Changed

- `internal/content` is now the public `content` package, with reader-based decoders
  (`DecodeManifest`, `DecodeFiles`, `DecodeRoleMeta`, `DecodeInstallInfo`) beneath the existing
  path-based functions. syft parsers are handed a reader, and `internal/` cannot be imported by an
  upstream contribution, so neither the cataloger nor a future syft PR could have reused a single
  line of the previous shape
- A nonexistent content root is now an error. It previously produced a valid, empty BOM, making a
  typo'd path indistinguishable from a control node with nothing installed
- An empty inventory now serialises `"components": []` rather than `null`, which the CycloneDX
  schema rejects
- README milestones are labelled M1–M5 rather than 0.1–0.4: those were never version numbers, and
  once v0.2.0 existed the collision was actively misleading

## [0.1.1] - 2026-07-31

Ships artifacts for the first time. No change to the tool's behaviour — every change here is to
what a release publishes and how.

### Added

- Release workflow: cross-compiled binaries for linux/darwin/windows on amd64 and arm64, a
  `SHA256SUMS` file, and a CycloneDX SBOM **per binary**, catalogued from the built artefact so it
  records the linked Go module graph and `pkg:golang/stdlib`
- SLSA build provenance and cosign signing of the checksum file, both gated on the repository
  being public — keyless signing publishes the repository identity to a public transparency log,
  which would defeat the point of keeping the repository private
- CI assertions on every released SBOM: it must carry Go components including `stdlib`, must not
  catalogue CI configuration, must not include the PE cataloger's filename-derived identifiers,
  and must contain no absolute build paths

### Changed

- Version is injected from the git tag at build time and defaults to `dev`, so an untagged local
  build cannot claim to be a release. **v0.1.0's binaries report `0.1.0` rather than `v0.1.0`**
  because that tag predates the change; v0.1.1 is the first release where injection takes effect
- Release SBOMs are catalogued from the binaries rather than the repository directory. The
  previous approach returned 23 components of which 16 were the GitHub Actions workflow files,
  and missed the Go standard library entirely
- Release SBOMs no longer embed the absolute path they were scanned from

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

[Unreleased]: https://github.com/jrjsmrtn/ansible-bom/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/jrjsmrtn/ansible-bom/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/jrjsmrtn/ansible-bom/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jrjsmrtn/ansible-bom/releases/tag/v0.1.0
