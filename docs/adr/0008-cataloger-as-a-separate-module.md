# 8. Keep the syft Cataloger Out of the CLI Binary

Date: 2026-07-31

## Status

Accepted — supersedes part of [ADR-0003](0003-go-with-a-syft-cataloger-strategy.md)

## Context

[ADR-0003](0003-go-with-a-syft-cataloger-strategy.md) chose Go on distribution grounds and stated
the delivery shape as *"a syft cataloger for the BOM half, **embedded in that binary** via
syft-as-a-library from the start"*. That was written before anyone measured what importing syft
costs, or looked at the interface a cataloger has to satisfy.

Both were measured on 2026-07-31.

**Dependency cost.** A binary that imports nothing but `syft/pkg`:

| | modules in the graph | binary |
|---|---|---|
| `ansible-bom` today | **3** | 2.9 MB |
| importing `syft/pkg` alone | **445** | 3.9 MB |

A 148× increase in dependency surface. For a tool whose purpose is telling people what is inside
their software, and whose own SBOM currently lists three components, that is not a neutral cost.

**And it buys `ansible-bom` users nothing.** The tool already emits CycloneDX natively. Embedding
syft so that `ansible-bom` can run our own cataloger against content it already parses directly is
circular. The cataloger's value is being *inside syft*, for syft's users — not inside a tool that
does not need it.

**The interface also does not fit the current code.** A syft cataloger is built from
`generic.NewCataloger(name).WithParserByGlobs(parser, globs...)`, where a parser has the shape:

```go
type Parser func(context.Context, file.Resolver, *Environment, file.LocationReadCloser) (
    []pkg.Package, []artifact.Relationship, error)
```

Parsers receive a **reader**; `ParseCollection`/`ParseRole` take a **directory path**. Reusing the
parsing logic requires decoders that operate on bytes, with the path-based functions as thin
wrappers.

## Decision

### 1. The CLI binary does not import syft

`cmd/ansible-bom` keeps its three-module dependency graph. Nothing in the shipped binary depends
on syft.

### 2. The parsers become a public, reader-based API

`internal/content` moves to `content`, and gains decoders that take bytes:

- `DecodeManifest`, `DecodeFiles` — collections
- `DecodeRoleMeta`, `DecodeInstallInfo` — roles

`ParseCollection(dir)`, `ParseRole(dir)` and `Scan(root)` remain, as filesystem wrappers over
those decoders. This is required regardless of where the cataloger lives: `internal/` cannot be
imported by anything outside this repository, so an upstream contribution could not reuse a single
line of it.

**Extended 2026-08-02: `internal/purl` moves to `purl` on the same argument.** The cataloger could
import it from `internal/` because it shares the module path prefix — Go's rule is lexical, and
`github.com/jrjsmrtn/ansible-bom/cataloger` sits under the parent of `internal/`. A package at
`github.com/anchore/syft/...` does not, so the constraint that applied to the parsers applied to
identifier construction too, and was simply not yet load-bearing. It becomes load-bearing the moment
an upstream contribution is attempted, which is the wrong time to discover it.

The cataloger's own import lags one release by necessity: it requires the parent at a *published*
version and resolves through the module proxy, so it can only name `purl/` once a tag contains it.
That is the same chicken-and-egg as the version pin itself, and is recorded at the call site.

### 3. The cataloger is a separate Go module

`cataloger/` carries its own `go.mod` and its own syft dependency. It is not built into the CLI,
does not affect the released binaries, and can be developed and tested against syft
independently — which is also the form in which it is easiest to port into syft's own tree, since
that is where an accepted cataloger would actually live.

**Alternatives rejected**:

| Alternative | Why not |
|---|---|
| Embed syft in the CLI, as ADR-0003 said | 3 → 445 modules for a capability the CLI does not need |
| A build tag on a single module | The dependency still enters `go.mod`, so the module graph and `go.sum` grow for every consumer regardless of tags |
| Duplicate the parsing logic in the cataloger | Two implementations of parsing that must agree about locale-formatted dates, placeholder manifest fields and version prefixes. They would diverge |
| Wait for [anchore/syft#5129](https://github.com/anchore/syft/issues/5129) before writing anything | ADR-0003 already settled this: the proposal is an accelerator, not a prerequisite. Writing it also makes the proposal concrete |

## Consequences

**Positive**:

- The shipped binary and its SBOM stay small and honest.
- The parsing logic has exactly one implementation, reachable by both the CLI and the cataloger.
- Reader-based decoders are what an upstream contribution needs anyway.
- The cataloger can track syft's API on its own schedule without touching the release artefacts.

**Negative**:

- Two modules in one repository: `go test ./...` at the root no longer covers everything, and CI
  must run the cataloger's tests separately.
- Promoting `content` to a public API means its shape is now something consumers can depend on,
  and 0.x is the only window in which to change it freely.
- The cataloger is unreleased and unreleasable as part of the binary, so it has no users until it
  is either upstreamed or published separately.
- **The module is not independently consumable, and the cause is repository privacy.** It depends
  on the parent through a `replace`, which is ignored when a module is consumed as a dependency,
  so `go get` of the cataloger fails on the placeholder version.

  This was first recorded as "no published tag contains `content/`", with the fix expected to be
  the next tag. **That was wrong.** Tried on 2026-07-31 immediately after tagging v0.2.0 — the
  first tag that does contain it — and the build still fails:

  ```
  verifying go.mod: .../ansible-bom@v0.2.0/go.mod:
    reading https://sum.golang.org/lookup/...@v0.2.0: 404 Not Found
    git ls-remote ...: could not read Username for 'https://github.com'
  ```

  `proxy.golang.org` and `sum.golang.org` cannot read a private repository, and Go verifies a
  required version's `go.mod` **even when a `go.work` workspace supplies the module locally** — so
  a workspace does not avoid it either. `GOPRIVATE` only helps someone already holding
  credentials, and an external consumer cannot fetch a private repository by any route.

  **Closed 2026-07-31** when the repository went public. The `replace` is gone and the parent is
  required at `v0.2.1`; verified with `GOWORK=off go build ./... && go test ./...` against the
  published module rather than the working tree.

  One consequence was not anticipated: **a `go.work` workspace cannot be committed.** It unifies
  the build list across its modules, so syft's `go >= 1.26.3` requirement propagates to the CLI
  module and breaks both its declared 1.23 floor and the CI job that proves it. The workspace
  remains a per-developer convenience, gitignored, documented in CONTRIBUTING.

  *Original text:* **The exit is therefore the `public-release` gate, not another tag.** Accepted until then,
  because the destination for this code is syft's own tree, where neither the `replace` nor the
  parent dependency travels with it.

**Risks**:

- *Interface drift.* syft's cataloger interface carries no stability guarantee. A separate module
  contains the damage but does not prevent it.
- *The proposal is declined.* Then the cataloger has no home, and the work is a sunk cost against
  a distribution advantage that never materialises — the risk ADR-0003 already records.

## References

- [ADR-0003](0003-go-with-a-syft-cataloger-strategy.md) — the Go decision, whose delivery shape
  this supersedes
- [anchore/syft#5129](https://github.com/anchore/syft/issues/5129) — the upstream proposal
- syft `syft/pkg.Cataloger` and `syft/pkg/cataloger/generic` (v1.50.0)
