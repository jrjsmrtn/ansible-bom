# Quality Configuration

> **Reference**: the single source of truth for toolchain versions, quality gates and formatting.
> For *why* these values were chosen, see [ADR-0002](../adr/0002-adopt-development-best-practices.md)
> and [ADR-0003](../adr/0003-go-with-a-syft-cataloger-strategy.md). This document records *what* is
> configured, and where.

**Perishable.** These values live in five files across two modules. When they drift, they drift
silently. Verify against the sources listed in each table.

---

## Go toolchain

Three different versions are in play, deliberately. Conflating them is the mistake this table
exists to prevent — it has already been made once, and shipped a build against a stdlib with known
advisories before CI caught it.

| Role | Version | Where | Meaning |
|---|---|---|---|
| **CLI floor** | `1.23.0` | `go.mod` | The oldest Go a *consumer* needs. A compatibility claim, nothing more |
| **Cataloger floor** | `1.26.3` | `cataloger/go.mod` | Inherited from syft, not chosen — syft v1.50.0 refuses anything older |
| **Build line** | `1.26` | workflow `go-version:` | What CI and the release actually build with |

### Why they differ

- The **floor** is what `go.mod` declares. From Go 1.21 the `go` directive is a hard minimum, so
  setting it high excludes users. It is set from evidence: the newest standard-library API the CLI
  uses is `strings.Cut` (Go 1.18), and 1.23.0 was verified by building and testing under a real
  downloaded `GOTOOLCHAIN=go1.23.0`.
- The **build line** must be current. Released binaries embed the standard library they were built
  with, and each binary's SBOM records `pkg:golang/stdlib@<version>` — so building at the floor
  would publish a known-vulnerable stdlib *and* document it in the accompanying BOM.
- The **cataloger floor** is not a choice. syft requires ≥ 1.26.3; attempting 1.24 fails outright.

### Per call site

Every `setup-go` in the repository, and why it reads what it reads:

| Workflow | Job | Setting | Intent |
|---|---|---|---|
| `ci.yml` | `test` | `1.26` | build and test what ships |
| `ci.yml` | **`floor`** | `go-version-file: go.mod` | **deliberately the minimum** — proves the compatibility claim is real |
| `ci.yml` | `cataloger` | `go-version-file: cataloger/go.mod` | syft's floor; reading the root would select 1.23 and fail to build |
| `ci.yml` | `bom` | `1.26` | |
| `ci.yml` | `vulnerabilities` | `1.26` | scanning the floor reports advisories about a version nothing is built with |
| `codeql.yml` | `analyze` | `1.26` | analyse what ships |
| `scorecard.yml` | `govulncheck` | `1.26` | as above; otherwise this fails every Monday |
| `release.yml` | `build` | `1.26` | **the one that matters most** — shipped artifacts |

### Maintenance obligation

The build line is **pinned, not `stable`**, so a new Go major release cannot change what CI
produces without a commit. Patch releases within the line are still taken, which is how stdlib
security fixes arrive.

**Nothing automates the major bump.** Dependabot does not manage `go-version`. Moving to `1.27` is
a deliberate act, roughly twice a year, and the failure mode is silent: the build keeps working on
an increasingly stale line until someone notices. When bumping, change every `go-version: "1.26"`
in `.github/workflows/` — the `floor` and `cataloger` jobs are intentionally excluded.

---

## Quality checks by stage

| Check | Local hook | CI | Scheduled | Source |
|---|---|---|---|---|
| Secret scanning (`gitleaks protect --staged`) | ✅ | — | — | `.lefthook.yml` |
| `gofmt -l` | ✅ | ✅ | — | `.lefthook.yml`, `ci.yml` |
| `go vet` | ✅ | ✅ | — | both |
| `go test` | ✅ | ✅ | — | both |
| `go test -race` + coverage | — | ✅ | — | `ci.yml` |
| Floor compatibility build | — | ✅ | — | `ci.yml` |
| Cataloger module tests | — | ✅ | — | `ci.yml` |
| Root module must not import syft | — | ✅ | — | `ci.yml` |
| CycloneDX schema conformance | — | ✅ | — | `ci.yml`, `scripts/validate-bom.py` |
| `govulncheck` | — | ✅ | ✅ weekly | `ci.yml`, `scorecard.yml` |
| Dependency review | — | ✅ PRs only | — | `ci.yml` *(public repos only)* |
| CodeQL | — | ✅ | ✅ weekly | `codeql.yml` |
| OpenSSF Scorecard | — | — | ✅ weekly | `scorecard.yml` *(public repos only)* |
| Release SBOM assertions | — | — | on tag | `release.yml` |

The local hook deliberately mirrors CI's first three checks exactly, so a difference in *what* is
checked can never explain "works locally, fails in CI".

Two checks are inert while the repository is private, and activate by themselves at the
public-release gate: **Scorecard** and **dependency review**. Both are gated with
`if: ${{ !github.event.repository.private }}`. SLSA provenance and cosign signing in `release.yml`
are gated the same way, for a different reason — keyless signing publishes the repository identity
to a public transparency log.

---

## Formatting

| Setting | Value | Applies to | Source |
|---|---|---|---|
| Indent style | tab | `*.go` | `.editorconfig`, enforced by `gofmt` |
| Indent style | space, 2 | `*.md`, `*.yml`, `*.yaml`, `*.json` | `.editorconfig` |
| Indent style | space, 4 | `*.py` | `.editorconfig` |
| Max line length | 100 | `*.md`, `*.py` | `.editorconfig` |
| End of line | `lf` | all | `.editorconfig` |
| Final newline | required | all | `.editorconfig` |
| Trailing whitespace | trimmed | all **except `testdata/**`** | `.editorconfig` |

`testdata/**` is exempt from whitespace normalisation on purpose: fixtures are captured from real
installed content and must survive byte-identical, or the parser tests stop testing reality.

---

## Validation checklist

- [ ] `.editorconfig` matches the Formatting table
- [ ] Every `go-version:` in `.github/workflows/` is the same pinned line
- [ ] The `floor` and `cataloger` jobs are the *only* ones reading a `go-version-file`
- [ ] `go.mod`'s floor still builds: `GOTOOLCHAIN=go1.23.0 go build ./... && go test ./...`
- [ ] The local hook and CI run the same formatting, vet and test commands
- [ ] `govulncheck` is clean on the build line
- [ ] The root module does not import syft: `go list -m all | grep -c anchore/syft` is `0`

## Related

- [ADR-0002](../adr/0002-adopt-development-best-practices.md) — practices and CI hardening policy
- [ADR-0003](../adr/0003-go-with-a-syft-cataloger-strategy.md) — the Go floor decision and evidence
- [ADR-0008](../adr/0008-cataloger-as-a-separate-module.md) — why the cataloger's toolchain differs
- [CONTRIBUTING.md](../../CONTRIBUTING.md)

_Last reviewed: 2026-07-31._
