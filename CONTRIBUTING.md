# Contributing

Thanks for considering a contribution. This document covers what you need to build the project and
what a change is expected to look like.

## Development setup

Requires Go — the version is declared in `go.mod`. No other toolchain is needed for the CLI.

```bash
git clone https://github.com/jrjsmrtn/ansible-bom
cd ansible-bom
go build ./...
go test ./...
```

Optional but recommended:

```bash
lefthook install    # the pre-commit gate: gitleaks, gofmt, go vet, go test
```

### The two modules

This repository holds **two** Go modules, which is deliberate:

| Module | Contents | Why separate |
|---|---|---|
| `.` (root) | the CLI and its packages | 3 dependencies, and it stays that way |
| `cataloger/` | syft catalogers | importing syft takes the graph to 445 modules |

`go test ./...` at the root **does not reach the cataloger**. Run its tests separately:

```bash
cd cataloger && go test ./...
```

The reasoning is in [ADR-0008](docs/adr/0008-cataloger-as-a-separate-module.md). The short version:
the shipped binary must not carry syft's dependency graph for a capability it does not use. CI
asserts the root module stays syft-free, so a stray import will fail the build rather than quietly
undo the decision.

`cataloger/` requires the root module at a **published version**, not through a `replace`. That
became possible when the repository went public: a `replace` is ignored when a module is consumed
as a dependency, and the module proxy cannot read a private repository. ADR-0008 records what was
tried before that.

Consequently, a local edit to `content/` is invisible to the cataloger until it is published. If
you are changing both, use a Go workspace:

```bash
go work init . ./cataloger
```

**`go.work` is deliberately not committed.** A workspace unifies the build list across its modules,
so syft's `go >= 1.26.3` requirement would propagate to the CLI module and break both its declared
1.23 floor and the CI job that proves it. Use `GOWORK=off` — or delete `go.work` — when building or
testing the CLI at its floor.

## Toolchain versions

Three Go versions are in play and they are not interchangeable — the floor in `go.mod` is what a
consumer needs, the pinned line in the workflows is what artifacts are built with, and the
cataloger inherits syft's. The
[quality configuration reference](docs/reference/quality-configuration.md) has the table and a
validation checklist you can run.

## Testing

- Go's standard `testing`, table-driven. No assertion library.
- Fixtures in `content/testdata/` are **captured from real installed content**, with provenance
  recorded in that directory's README. Do not tidy them. They are shaped like reality, including
  the parts of reality that are ugly — placeholder manifest fields, locale-formatted dates,
  inconsistently prefixed versions.
- To run the parser against a real control node:

  ```bash
  ANSIBLE_BOM_REAL_TREE=/path/to/content go test ./content -run RealTree -v
  ```

## What a change should look like

- **`gofmt` is non-negotiable**, `go vet` clean, tests passing.
- **[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)**. Scopes follow the
  subcommands and parsers: `feat(lock):`, `fix(roles):`, `feat(cyclonedx):`.
- **Branch from `develop`**, not `main`. `main` tracks releases.
- Anything that changes **what the output asserts** needs an ADR in `docs/adr/`, not just a commit
  message. See [ADR-0001](docs/adr/0001-record-architecture-decisions.md) for what qualifies.

## The one rule that matters most

**Never let the output claim more than it knows.**

This project exists because bills of materials routinely overstate their own assurance, and most
of its design decisions are about refusing to do that:

- Roles carry no checksums, so they are reported as *unverifiable* — never as verified.
- No vulnerability database indexes Ansible content, so every component states its coverage
  status. An empty scanner result must never read as "clean".
- Content that cannot be parsed is *reported*, and the BOM declares itself incomplete.
- A component with no recorded version stays unversioned rather than getting a placeholder.

A change that makes output look more confident than the underlying data supports will be declined,
however convenient it is. If you find a case where the tool already does this, that is a bug worth
reporting — see [SECURITY.md](SECURITY.md), which treats overstated assurance as a security issue.

## Reporting bugs vs proposing features

- **Bug**: use the bug report template. A real content tree that reproduces it is worth more than
  anything else you can provide — scrub anything internal first.
- **Feature**: open an issue before writing code, especially for anything touching the output
  contract. Some things are deliberately out of scope: dependency resolution, installing content,
  and modelling what playbooks deploy on managed hosts. Those are recorded in the
  [SPARK analysis](docs/inception/spark-analysis.md) with reasons.
- **Security**: do not open an issue. See [SECURITY.md](SECURITY.md).

## Licence

By contributing you agree that your contributions are licensed under
[Apache-2.0](LICENSE), and that new source files carry the SPDX header used throughout:

```go
// SPDX-FileCopyrightText: <year> <you>
// SPDX-License-Identifier: Apache-2.0
```
