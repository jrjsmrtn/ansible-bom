# ansible-bom

Inventory installed Ansible content — collections **and** legacy roles — then emit a resolved
lockfile, a drift report against `requirements.yml`, and a CycloneDX BOM.

Not affiliated with or endorsed by Red Hat, Inc. "Ansible" is a trademark of Red Hat, Inc.

## Project Context

- **Category**: Development
- **Type**: CLI tool (plus a syft cataloger, see ADR-0003)
- **Stack**: Go
- **License**: Apache-2.0
- **Tier**: t1
- **Distribution profile**: Private — **Public once stable** (ships-artifacts: not yet; released
  binaries will set the marker)

## Why this exists

Ansible Galaxy has no lockfile and no mainstream SBOM generator catalogs Ansible content. The
consequence is sharper than "no BOM": with unpinned content, a byte-identical playbook executes
through *different module code* between runs, which voids Ansible's idempotency guarantee at the
one layer nobody inspects. See [the SPARK analysis](docs/inception/spark-analysis.md) — it is the
design document, and it is measured against a real controller rather than assumed.

Lead with idempotency and reproducibility. The BOM is what the compliance-driven audiences need;
the lockfile is what makes the tool worth running daily.

## Project Tier

Current tier: **t1**

Artifacts: `CLAUDE.md`, `README.md`, `CHANGELOG.md`, `LICENSE`, `.gitignore`, `.editorconfig`,
foundation + decision ADRs in `docs/adr/`, pre-commit gate via `.lefthook.yml`, `main` +
`develop` branches.

Promotion triggers:

- **t1 → t2** when the documentation outgrows the README — likely at first public release, since
  the audience registry already identifies tutorial, how-to and reference needs across six
  audiences. t2 adds the Diátaxis tree, a roadmap, C4, and sprint cadence.
- **Private → Public** is the `public-release` gate, expected before v1.0. The repository is kept
  publishable from the first commit: **no internal identifiers, hostnames, or paths anywhere in
  history.**

## Release Gating

Two independent gates — do not conflate them:

| Gate | Condition |
|---|---|
| Public release | the tool is stable |
| **v1.0** | the `ansible` purl type is **approved and implemented** ([purl-spec#854](https://github.com/package-url/purl-spec/pull/854)) |

Everything before that is **0.x with provisional identifiers**, labelled as such in the output.
See ADR-0004.

## Scope Discipline

Out of scope, decided at inception — do not drift into these:

- **Dependency resolution.** The tool observes what `ansible-galaxy` did. It never resolves.
- **Installing anything.** Read-only against the filesystem.
- **Vulnerability matching.** No Ansible ecosystem exists in OSV — confirmed by POC-2, and an
  unregistered purl returns an empty result rather than an error. Never imply a scan happened
  (ADR-0006).
- **OBOM of managed hosts.** What playbooks *deploy* is a different, harder document.

Deferred (not rejected): collection signature verification, Execution Environment images as an
input, TEA publishing.

## Foundational ADRs

Read these at the start of each session.

| ADR | Purpose | Summary |
|-----|---------|---------|
| [0001](docs/adr/0001-record-architecture-decisions.md) | HOW TO DECIDE | Decision methodology |
| [0002](docs/adr/0002-adopt-development-best-practices.md) | HOW TO DEVELOP | Go practices, testing, quality gates |
| [0003](docs/adr/0003-go-with-a-syft-cataloger-strategy.md) | WHAT TECH | Go, two delivery surfaces, Apache-2.0 |
| [0004](docs/adr/0004-provisional-purl-identifiers.md) | IDENTITY | `pkg:ansible` provisional; 1.0 gate |
| [0005](docs/adr/0005-two-tier-collection-and-role-model.md) | DATA MODEL | Roles carry no checksums; say so |
| [0006](docs/adr/0006-declare-vulnerability-coverage-status.md) | OUTPUT CONTRACT | No OSV coverage, and it fails silently; label it |

## Development Practices

- **Testing**: Go standard `testing`, table-driven; fixtures in `testdata/` captured from real
  content trees
- **Versioning**: SemVer; 0.x during development (see Release Gating)
- **Git workflow**: Gitflow — `main`, `develop`, `feature/*`, `release/*`, `hotfix/*`
- **Commits**: Conventional Commits
- **Changelog**: Keep a Changelog

## Quick Commands

```bash
go build ./...
go test ./...
gofmt -l .
go vet ./...
lefthook run pre-commit
```

## AI Collaboration Notes

**What AI should know:**

- The SPARK analysis is authoritative for scope and rationale. Read it before proposing features.
- **Roles and collections are not equivalent.** Collections carry `MANIFEST.json` + `FILES.json`
  with sha256 per file; roles carry no checksums and a version string in
  `meta/.galaxy_install_info`. Never emit them as if they had equal assurance (ADR-0005).
- Author-supplied manifest fields are untrustworthy — a real collection was observed carrying the
  unedited Galaxy skeleton placeholder as its `repository`. Use only namespace, name and version
  for identity.
- `.galaxy_install_info` records `install_date` as a **locale-formatted** string. Treat it as
  opaque; do not parse it.
- Role versions are inconsistently prefixed (`v0.3.2` alongside `3.5.0`). Normalise defensively.
- purl construction lives in **exactly one function** so the 1.0 identity change is one edit.
- `ansible-galaxy` has no plugin mechanism — there is no subcommand to extend. This is why the
  tool is standalone.

**AI leads**: parsers, table-driven tests, fixture capture, CycloneDX emission.
**Human leads**: identity decisions, upstream engagement with syft, scope boundaries, what goes
public.
