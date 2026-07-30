# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Versioning note**: 0.x carries provisional component identifiers. **1.0 is gated on the
`ansible` purl type being approved and implemented upstream** — see
[ADR-0004](docs/adr/0004-provisional-purl-identifiers.md).

## [Unreleased]

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
- Upstream cataloger proposal opened at anchore/syft#5129
- Confirmed from `ansible-core` 2.20.0 source that `ansible-galaxy` has no extension point,
  upgrading a negative-evidence finding to a verified one
