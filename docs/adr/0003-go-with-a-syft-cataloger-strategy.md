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

- **`ansible-galaxy` is not extensible.** Ansible's plugin types serve playbook execution; the
  Galaxy CLI is a fixed subcommand set in `ansible-core` with no documented extension point. There
  is no `ansible-galaxy lock` plugin path. *(Negative evidence — see Consequences.)*
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
- **The `ansible-galaxy` non-extensibility finding is negative evidence** — absence of any
  documented hook, not a statement that extension is unsupported. If a mechanism exists and was
  missed, the Python option becomes materially stronger and this ADR should be revisited.

**Risks**:

- *Upstream rejection or stall.* The strategic value is syft's reach; if that path closes, the
  standalone binary still works but the advantage over Python largely evaporates. Mitigate by
  opening a proposal issue **before** the code shape is fixed.
- *Go version floor.* `go.mod` currently declares the toolchain version in use. Lower it to the
  oldest supported Go before public release; requiring the newest release would exclude the
  enterprise CI this targets.

## References

- [SPARK analysis](../inception/spark-analysis.md) — technology options, distribution reasoning
- [syft](https://github.com/anchore/syft) — cataloger interface and examples
- [anchore/syft#5129](https://github.com/anchore/syft/issues/5129) — the upstream proposal, opened 2026-07-31. Its outcome determines whether the distribution advantage this ADR rests on is actually available
- [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0)
