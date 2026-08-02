# Vendored snapshot — NOT the specification

`ansible-definition.json` is a verbatim copy of a **proposed, unmerged** purl type definition.
It is test input, not an authority, and it must not be cited as though the `ansible` purl type
exists. It does not.

| | |
|---|---|
| Source | [purl-spec#854](https://github.com/package-url/purl-spec/pull/854), *Add new PURL type: 'ansible'* |
| Author | `anweshadas` |
| Fetched from | `anweshadas/purl-spec` at `b82de5e434126e675b78bac11c3c1f7b2d03759a` |
| Path upstream | `types/ansible-definition.json` |
| Snapshot taken | 2026-07-31 |
| sha256 | `72e84215467439c9753d610fc4bf737cb5aad9d2d6ade17cb2d1675927cff5e9` |
| PR state when taken | open, `CHANGES_REQUESTED`, no activity since 2026-06-09 |

It declares `$schema: purl-type-definition.schema-1.0.json`, so the constraints below are
machine-readable rather than prose: `namespace_definition.requirement`, `name_definition`,
`version_definition`, a `qualifiers_definition` list, and seven canonical `examples`.

## Why it is here

`conformance_test.go` asserts what this tool emits **against** this file. The test currently
records a **divergence**, not conformance — see ADR-0004. That is deliberate: the divergence is a
fact about the world, and pinning it means any change on either side fails a test instead of going
unnoticed. Before this file existed, ADR-0004 paraphrased the proposal in prose and the paraphrase
was wrong for months, because there was nothing to check it against.

## Refreshing it

Only refresh deliberately, and expect the conformance test to fail — that failure is the signal.

```bash
SHA=$(gh api repos/package-url/purl-spec/pulls/854 --jq .head.sha)
gh api "repos/anweshadas/purl-spec/contents/types/ansible-definition.json?ref=$SHA" --jq .content \
  | base64 -d > purl/testdata/purl-spec-854/ansible-definition.json
```

Then update the table above, re-read the diff, and decide — in ADR-0004 — whether the divergence
still stands or the tool should now conform. Do not silently update the expectations to match.

Refreshing is also the moment to re-read
[Findings for purl-spec#854](../../../../docs/reference/upstream-purl-findings.md), which quotes
this snapshot's wording and is written against it.

If the PR is merged, this directory is replaced by the real definition from `package-url/purl-spec`
and the test becomes a genuine conformance test rather than a divergence record.
