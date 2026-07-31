# 3. Use Go, with a syft Cataloger as the Distribution Strategy

Date: 2026-07-31

## Status

Accepted

## Context

The tool has two capability halves with different natural homes:

- **BOM generation** from an installed content tree — the same job every SBOM generator does, for
  an ecosystem none of them covers.
- **Lockfile and drift** against `requirements.yml` — an Ansible *workflow* concern that no SBOM
  generator would host.

The primary audiences (enterprise IaC engineers, OEM delivery teams, packagers, SecOps) already
run SBOM tooling in CI. A tool they must separately adopt starts at a disadvantage, so the
decisive question is **distribution**, not language ergonomics.

Two findings from the [SPARK analysis](../inception/spark-analysis.md) constrain the answer:

- **`ansible-galaxy` is not extensible.** Confirmed against `ansible-core` 2.20.0 source
  (2026-07-31), not inferred from missing documentation:
  - `cli/galaxy.py:init_parser()` constructs every subcommand with an explicit `add_parser()`
    call — `collection`, `role`, and the actions `download`, `init`, `remove`, `delete`, `list`,
    `search`, `import`, `setup`, `info`, `verify`, `install`, `build`, `publish`. No registry, no
    iteration, no discovery.
  - Dispatch is `set_defaults(func=self.execute_<action>)` — bound methods resolved when the
    parser is built. Nothing is looked up by name at runtime.
  - `galaxy.py` imports no plugin loader, `entry_point`, `importlib.metadata` or `pkg_resources`
    machinery.
  - `ansible-core` declares only `console_scripts` entry points; there is no extension group.
  - `ansible/plugins/` contains action, become, cache, callback, cliconf, connection,
    doc_fragments, filter, httpapi, inventory, lookup, netconf, shell, strategy, terminal, test,
    vars — every one a playbook-execution concern. None is CLI- or command-related.
  - **The one galaxy-named plugin type is a false lead**: `galaxy_server` is config-only. It
    defines *which servers to talk to* via `[galaxy_server.<name>]` ini sections; there is no
    `plugins/galaxy_server/` directory and it contributes no behaviour.
- **syft is extensible**, via a documented cataloger interface (`syft/cataloger`,
  `generic.Cataloger`, and a `create_custom_sbom` example), and ships 61 package catalogers
  including IaC precedents — `terraform-lock-cataloger`, `github-actions-usage-cataloger`,
  `wordpress-plugins-cataloger`. Ansible is absent.

## Decision

**Go**, with **two delivery surfaces from one codebase**:

1. A **standalone `ansible-bom` binary** carrying every capability — `lock`, `drift`, `scan`,
   `verify`.
2. A **syft cataloger** for the BOM half, embedded in that binary via syft-as-a-library from the
   start, and proposed upstream in parallel.

The library path means the cataloger ships immediately and depends on nobody's approval;
upstreaming is an accelerator, not a prerequisite.

**Cataloger design follows syft's `declared`/`installed` tag convention**, which already models
the distinction this project is built on:

| Cataloger | Tag | Source |
|---|---|---|
| `ansible-requirements-cataloger` | `declared` | `requirements.yml` |
| `ansible-collection-cataloger` | `installed` | `ansible_collections/` tree |
| `ansible-role-cataloger` | `installed` | roles tree |

Note that the closest precedent, `terraform-lock-cataloger`, is `declared`-only — syft cannot see
what Terraform actually installed. Ansible content sits on disk with checksums, so this cataloger
can cover both sides, which is better coverage than the precedent achieves. That is the pitch for
the upstream proposal.

**Licence: Apache-2.0.** It follows from the strategy rather than from preference: syft is
Apache-2.0, and code intended for possible upstream contribution is far easier to donate when it
is already under the destination licence. Apache-2.0's explicit patent grant also suits the
enterprise adopters who are the primary audience. This overrides the MIT default used elsewhere in
this author's projects.

**Alternatives considered**:

| Option | Pros | Cons | Decision |
|---|---|---|---|
| **Go — standalone binary + syft cataloger** | reaches users inside the tool they already run; single static binary suits CI, air-gapped mirrors and packagers; SPDX output and grype integration come free via syft; one language for both surfaces | less maintainer fluency than Python; Go's CycloneDX library is less mature than Python's | **Selected** |
| Python — standalone CLI | installs beside `ansible-core`; `cyclonedx-python-lib` is the reference implementation; fastest to write; closest to the Ansible ecosystem's own language | no syft/grype integration; another tool to adopt; **the Ansible-adjacency advantage is illusory** — with no `ansible-galaxy` extension point, a Python tool is exactly as separate as a Go binary, while being harder to distribute into CI | Rejected |
| Both — Go cataloger plus Python CLI | each half in its most natural language | two codebases, two parsers for the same messy metadata, guaranteed divergence | Rejected |

## Consequences

**Positive**:

- Ansible coverage can appear in a tool enterprises already run, at zero adoption cost.
- Single static binary: no runtime to reconcile with `ansible-core`'s Python environment.
- One parser implementation serves both surfaces.
- Apache-2.0 removes a licensing obstacle from the upstream path before it arises.

**Negative**:

- Slower initial development than Python would have been. Accepted deliberately.
- A less mature CycloneDX library; some emission may need hand-rolling.
- Coupling to syft's cataloger interface, which is not a stability-guaranteed API — a breaking
  change upstream would mean adapter work.
- Extending `ansible-galaxy` is not merely undocumented but structurally unavailable, so the
  lockfile and drift capabilities must ship as a separate tool no matter which language is
  chosen. This closes the caveat this ADR originally carried: the Python option cannot be
  rehabilitated by an extension point, because none exists.

**Risks**:

- *Upstream rejection or stall.* The strategic value is syft's reach; if that path closes, the
  standalone binary still works but the advantage over Python largely evaporates. Mitigate by
  opening a proposal issue **before** the code shape is fixed.
- ~~*Go version floor.*~~ **Closed 2026-07-31, at the public-release gate.**

  `go.mod` had declared `go 1.26.5` — simply the toolchain that happened to be installed — which
  would have excluded the enterprise CI this project targets, since a `go` directive is a hard
  minimum from Go 1.21 onwards.

  The floor was chosen from evidence rather than taste. The newest standard-library API the CLI
  uses is `strings.Cut` (Go 1.18); nothing depends on `slices`, `maps`, `errors.Join`,
  range-over-int or the `min`/`max` builtins. **`go 1.23.0`** was set and then *verified against a
  real toolchain* — `GOTOOLCHAIN=go1.23.0`, downloaded, with the full suite building and passing —
  rather than assumed from an API survey. That is two releases below Go's own support window,
  which is the margin lagging CI needs.

  **The two modules deliberately diverge.** `cataloger/` declares `go 1.26.3`, because syft
  v1.50.0 requires it; attempting 1.24 fails with `requires go >= 1.26.3`. That floor is inherited,
  not chosen, and it is a further argument for the split in
  [ADR-0008](0008-cataloger-as-a-separate-module.md): were the cataloger inside the CLI module, its
  dependency would drag every consumer of the CLI onto a toolchain three releases newer, for a
  capability the CLI does not use.

  Consequence for CI: the cataloger job reads `cataloger/go.mod` rather than the root one. Reading
  the root would select 1.23 and fail to build syft.

  **A second consequence, caught by CI immediately after the change.** The declared floor is what
  a *consumer* needs; it is not the toolchain to *build* with. Every workflow read
  `go-version-file: go.mod`, so lowering the floor silently moved CI and the release build onto
  go1.23.0 — which carries `GO-2026-4602` and `GO-2025-3750` in `os`, a package this code calls
  constantly. `govulncheck` failed the build and said so.

  Released binaries embed the standard library they are built with, and each binary's own SBOM
  records `pkg:golang/stdlib@<version>`, so shipping the floor would have published a known-
  vulnerable stdlib *and* recorded the fact in the accompanying BOM. Builds now use
  `go-version: stable`; a separate `floor` job builds and tests at exactly the declared minimum,
  proving the compatibility claim without treating an old toolchain as shippable.

## References

- [SPARK analysis](../inception/spark-analysis.md) — technology options, distribution reasoning
- [syft](https://github.com/anchore/syft) — cataloger interface and examples
- [anchore/syft#5129](https://github.com/anchore/syft/issues/5129) — the upstream proposal, opened 2026-07-31. Its outcome determines whether the distribution advantage this ADR rests on is actually available
- [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0)
