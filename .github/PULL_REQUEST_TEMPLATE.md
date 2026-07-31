## What this changes

<!-- and why -->

## Type

- [ ] Bug fix
- [ ] New capability
- [ ] Documentation
- [ ] Build, CI or release tooling

## Checklist

- [ ] `gofmt -l .` is empty, `go vet ./...` clean
- [ ] `go test ./...` passes — and `cd cataloger && go test ./...` if that module changed
- [ ] Branched from `develop`
- [ ] `CHANGELOG.md` updated under `[Unreleased]`
- [ ] New source files carry the SPDX header

## Output contract

- [ ] This does **not** make the output claim more than the underlying data supports
- [ ] If it changes what the output asserts, an ADR is included in `docs/adr/`
