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

A `post-merge` hook runs `scripts/sync-remotes.sh`, which pushes the current branch to every
remote that already tracks it. This repository has more than one remote, and a pull request
merged on one forge lands only there — after which `git pull` against the stale remote reports
"Already up to date", which is true and thoroughly misleading. A hook cannot observe a merge that
happened remotely, so it fires when the merge *arrives* locally and pushes it onward.

It is fast-forward only, never forces, only touches branches a remote already has, and always
exits 0 — a remote being unreachable must not fail a merge that already succeeded. It names no
remote and no host, so it is safe in a public repository and works on whatever remotes are
configured.

CI runs the same checks — `gofmt`, `go vet`, `go test` — so a difference in *what* is checked can
never be the reason something works locally and fails in CI. It adds what a hook cannot:
`go test -race`, `govulncheck`, and CycloneDX schema conformance for emitted BOMs.

**GitHub Actions hardening** (`.github/workflows/`):

- **Every action is SHA-pinned** to a full 40-character commit, with a `# vX.Y.Z` comment. A
  mutable tag can be repointed upstream at any time. **One exception**, now that the project
  ships artifacts: `slsa-framework/slsa-github-generator` is referenced by tag, because it
  verifies its own release tag and a SHA reference breaks that check. It is the only tag
  reference in `.github/workflows/`, and the validation grep excludes it by name.
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

**Several supply-chain controls only function on a public repository**, so the private phase runs
a reduced but honest set rather than a set that appears to run and does not. Both were verified by
running them, not assumed:

| Control | Private | Reason |
|---|---|---|
| OpenSSF Scorecard | gated off | needs GraphQL access the default token lacks on private repos |
| Dependency review | gated off | needs GitHub Advanced Security on private repos |
| SLSA provenance | gated off | keyless signing publishes the repository and workflow path to a **public** transparency log |
| cosign signing | gated off | same — Rekor is public, and a private repository's identity would leak into it |
| Everything else | runs | build, gofmt, vet, race tests, govulncheck, BOM conformance, release binaries, checksums, self-SBOM |

The provenance and signing gates deserve emphasis because the reason differs from the other two:
those simply *cannot* run while private, whereas keyless signing *would* work and is withheld on
purpose. Sigstore records the signing identity — repository name and workflow path — in Rekor, a
public append-only log. Signing a private repository's artifacts would publish exactly what
keeping it private protects. The gate lifts at the `public-release` gate, where signing becomes
both possible and the point.

Both carry `if: ${{ !github.event.repository.private }}` and activate by themselves at the
`public-release` gate, so neither is a step to remember.

**OpenSSF Scorecard is gated on the repository being public**, and is inert until then. Verified
by running it: on a private repository the action fails with `Resource not accessible by
integration`, because it queries the GitHub GraphQL API which the default `GITHUB_TOKEN` cannot
reach for private repos. The documented workaround is a personal access token — a long-lived
secret added to CI solely to grade a repository whose results cannot be published while private.
That trade is not worth it, so the job carries `if: ${{ !github.event.repository.private }}` and
starts working by itself at the `public-release` gate.

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
