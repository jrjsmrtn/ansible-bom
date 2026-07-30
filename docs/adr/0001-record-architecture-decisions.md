# 1. Record Architecture Decisions

Date: 2026-07-31

## Status

Accepted

## Context

This project is intended for public release and carries decisions that will be hard to revisit
once anyone depends on its output — component identity above all. It also began with a substantial
[SPARK analysis](../inception/spark-analysis.md) whose conclusions need somewhere durable to live,
rather than being rediscovered by reading a design document end to end.

The project is developed with AI assistance, where each session starts without memory of the last.
Decision rationale that exists only in a conversation is lost by the next session.

## Decision

We will use Architecture Decision Records for decisions that shape the tool's behaviour, its
output contract, or its relationship with upstream projects.

**Location**: `docs/adr/`. **Format**: Michael Nygard's — Context, Decision, Consequences — with
Status, and an alternatives table wherever a real choice existed. **Titles**: `# N. Title`
(adr-tools format), required for Structurizr `!adrs` integration should a C4 model appear at t2.
**Numbering**: sequential four digits, no gaps, never reused. **Index**: `index.yml` — not a
README, because the adr-tools importer parses every `.md` file in the directory and fails on
non-ADR markdown.

**What warrants an ADR**:

- Anything affecting the **output contract** — identifiers, schema, what a component means
- Upstream strategy (syft, purl-spec) and anything creating an external dependency
- Scope boundaries, especially reversals of the inception scope
- Licensing and distribution decisions
- Technology choices that would be expensive to unwind

**Relationship to the SPARK analysis**: the analysis is the *design document* — evidence,
alternatives, risk. ADRs are the *decision record*. Where they overlap, the ADR states the
decision and links to the analysis for evidence rather than restating it. The analysis is a
living document up to first release; ADRs are append-only, superseded rather than edited.

## Consequences

**Positive**:

- Decisions survive the session and the contributor.
- Settled questions stop being reopened; the alternatives tables record what was already rejected.
- A public contributor can reconstruct intent from the repository alone.

**Negative**:

- Writing overhead per significant decision.
- Two documents can drift — the analysis and the ADRs — if the boundary above is not respected.
- Judgement is needed about "significant"; the list is a heuristic.

## References

- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) — Michael Nygard
- [adr.github.io](https://adr.github.io/)
- [SPARK analysis](../inception/spark-analysis.md)
