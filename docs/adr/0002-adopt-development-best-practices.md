# 2. Adopt Development Best Practices

Date: 2026-07-31

## Status

Accepted

## Context

`ansible-bom` is a Go CLI intended for public release, for use in enterprise CI pipelines, and
possibly for partial contribution upstream to syft. Those three facts set the bar: the code must
be idiomatic enough to be accepted upstream, the release process trustworthy enough for CI
adoption, and the repository publishable from the first commit.

The project is a parser at heart. Its correctness risk is concentrated in reading messy real-world
metadata — locale-formatted dates, inconsistent version prefixes, placeholder manifest fields,
four different install sources — not in algorithmic complexity.

## Decision

### 1. Testing

- **Framework**: Go standard `testing`. No assertion library unless one earns its place.
- **Style**: table-driven, which suits a parser with many input shapes.
- **Fixtures**: `testdata/` trees captured from **real** installed content, not hand-written
  ideals. The inception analysis found placeholder `repository` fields, locale-formatted install
  dates and `v`-prefixed versions in a single real controller; synthetic fixtures would have
  missed all three.
- **Fixture hygiene**: captured fixtures must be scrubbed of any identifying detail before
  committing — see §7.
- **Coverage**: aim >80% on parsing and emission. Not a gate; a smell detector.

### 2. Versioning

[SemVer](https://semver.org/). 0.x during development. **1.0 is gated on an external condition**
— see [ADR-0004](0004-provisional-purl-identifiers.md) — not on feature completeness.

### 3. Git workflow

Gitflow: `main`, `develop`, `feature/*`, `release/*`, `hotfix/*`. `main` tracks releases; `develop`
is the integration branch. Chosen over trunk-only because this project ships versioned artifacts
to strangers, unlike an internal documentation repository.

**Commits**: [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) —
`<type>[optional scope]: <description>`, with the body carrying the *why*. Anything affecting the
output contract graduates to an ADR.

Types used here:

| Type | Use |
|---|---|
| `feat` | new capability |
| `fix` | bug fix |
| `docs` | documentation only |
| `refactor` | behaviour-preserving change |
| `perf` | performance |
| `test` | tests and fixtures |
| `build` | build system, Go module, release tooling |
| `ci` | pipeline configuration |
| `chore` | housekeeping with no src/test change |
| `revert` | reverts a previous commit |

Breaking changes are marked with `!` after the type/scope (`feat!:`) **and** a `BREAKING CHANGE:`
footer — the footer is what tooling reads reliably.

**Interaction with the gated 1.0** ([ADR-0004](0004-provisional-purl-identifiers.md)): the usual
`feat` → MINOR, `fix` → PATCH, `BREAKING CHANGE` → MAJOR mapping does not apply during 0.x, where
SemVer permits breaking changes in a minor bump. Mark them anyway. The accumulated
`BREAKING CHANGE:` footers are how the 1.0 release notes get written, and identifiers are expected
to break at least once before then.

Scopes follow the subcommands and the parsers — e.g. `feat(lock):`, `fix(collections):`,
`fix(roles):`, `feat(cyclonedx):`, `feat(cataloger):`.

### 4. Change documentation

[Keep a Changelog](https://keepachangelog.com/) in `CHANGELOG.md`, updated with each version bump.

### 5. Code conventions

- `gofmt` is non-negotiable; `go vet` clean.
- Effective Go and the Go Code Review Comments conventions apply — this code may be proposed
  upstream, where idiom is a review criterion.
- **purl construction lives in exactly one function.** The 1.0 identity change must be one edit.
- **No parsing of `install_date`.** It is a locale-formatted string; treat it as opaque.
- Version strings are normalised defensively (`v0.3.2` and `3.5.0` both occur in practice).
- Only namespace, name and version are trusted for identity; all other manifest fields are
  informational (placeholder values observed in real content).

### 6. Quality automation

Single pre-commit stage via lefthook — the checks are fast enough that splitting into two stages
would add ceremony without saving time:

- `gitleaks protect --staged` — secret scanning
- `gofmt -l` — formatting
- `go vet` — static analysis
- `go test ./...` — the suite is a parser test suite and will stay quick

CI runs the same checks — `gofmt`, `go vet`, `go test` — so a difference in *what* is checked can
never be the reason something works locally and fails in CI. It adds what a hook cannot:
`go test -race`, `govulncheck`, and CycloneDX schema conformance for emitted BOMs.

**GitHub Actions hardening** (`.github/workflows/`):

- **Every action is SHA-pinned** to a full 40-character commit, with a `# vX.Y.Z` comment. A
  mutable tag can be repointed upstream at any time. There is currently **no exception** to this
  rule; the one the ecosystem sanctions — `slsa-framework/slsa-github-generator`, which verifies
  its own release tag — does not apply until this project ships artifacts.
- **`permissions: contents: read`** at the top of every workflow, elevated per job only where
  genuinely needed. Only the Scorecard job elevates, for SARIF upload and OIDC.
- **`persist-credentials: false`** on every checkout, so no token is left on disk.
- **Pinned toolchains**: Go from `go.mod`, `govulncheck` at an exact version, `jsonschema` at an
  exact version. No `curl | sh`, no `latest`.
- **Dependabot** keeps both the Go modules and the SHA pins current, with a **7-day cooldown**:
  pinning and an update bot are one control, and a bot that adopts a release within minutes of
  publication would deliver a compromise faster than a human. Security updates bypass the
  cooldown by default.
- `concurrency` cancellation and `timeout-minutes` on every job; a runaway job holds a token.

**Known gap**: the Python dependency for BOM validation is version-pinned but not hash-pinned.
`--require-hashes` needs a generated lockfile covering platform-specific wheels; claiming it
before that exists would assert a control we do not have.

`govulncheck` also runs on a weekly schedule, because a branch that is not seeing commits can
become vulnerable without anyone touching it.

### 7. Publishability from the first commit

The repository is Private now and Public before v1.0. Rewriting history to remove a leak is
unpleasant and unreliable, so the constraint applies from commit one:

- No internal hostnames, usernames, paths, or network identifiers — anywhere, including test
  fixtures and commit messages.
- Documentation examples use reserved ranges: RFC 5737 for IPv4, RFC 2606 / `.example` / `.test`
  / `.invalid` for domains, RFC 7042 for MAC addresses.
- `gitleaks` enforces the secret half; the identifier half is a review responsibility.

### 8. Licensing

Apache-2.0 — rationale in [ADR-0003](0003-go-with-a-syft-cataloger-strategy.md), since it follows
from the upstream strategy. REUSE/SPDX headers are deferred to the `public-release` gate.

## Consequences

**Positive**:

- Fixtures from real trees mean the parser is tested against the failure modes that actually
  exist.
- Upstream-compatible idiom keeps the syft contribution path open.
- The publishability constraint costs nothing now and avoids a history rewrite later.

**Negative**:

- Gitflow is heavier than needed for a single contributor today.
- Capturing and scrubbing real fixtures is more work than writing synthetic ones.
- A coverage target with no gate relies on discipline.

## References

- [SPARK analysis](../inception/spark-analysis.md) — the metadata findings driving §5
- [Effective Go](https://go.dev/doc/effective_go) ·
  [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) ·
  [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
  [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html)
- [gitleaks](https://github.com/gitleaks/gitleaks) · [lefthook](https://github.com/evilmartians/lefthook) ·
  [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
