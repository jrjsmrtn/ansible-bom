# 9. Keep the Backlog In-Repo

Date: 2026-07-31

## Status

Accepted

## Context

Going public raises a question that is easy to answer by accident: where does the work live, and
what can a contributor see and claim?

This project has an in-repo backlog — the M1–M5 roadmap in `README.md`, the scope boundaries and
open questions in the [SPARK analysis](../inception/spark-analysis.md), and this ADR log. None of
it is on a tracker.

The observed failure mode at this gate is not choosing wrongly; it is **not choosing** — toggling
the Projects board on, seeding nothing, and leaving contributors to infer the backlog from prose.

The state at the gate:

- **One maintainer.** No one is competing for work or needs to claim it.
- **M1–M4 are done.** The roadmap is a record of completed work more than a queue.
- **M5 and v1.0 are both blocked upstream** — on [anchore/syft#5129](https://github.com/anchore/syft/issues/5129)
  and [purl-spec#854](https://github.com/package-url/purl-spec/pull/854). Neither is work anyone
  here can pick up; they are waits.
- **The remaining known work is small and recorded** in ADR consequences and the analysis's open
  questions.

Seeding a tracker from that produces perhaps three issues, two of which say "waiting for someone
else's decision".

## Decision

**The backlog stays in-repo. The tracker stays thin.**

Concretely:

- `README.md`'s milestone table remains the roadmap, and the [SPARK analysis](../inception/spark-analysis.md)
  remains authoritative for scope and for what is deliberately excluded.
- The issue tracker is for **incoming** work — bug reports and feature proposals from users — not
  as a mirror of the roadmap.
- **No Projects board.** A board over three issues is ceremony.
- **No milestones seeded** from M1–M5. They describe delivered work and two upstream waits, not a
  queue.

Labels exist to support the templates and release-note categorisation rather than to organise a
backlog: `bug`, `enhancement`, `breaking`, `ci`, `build`, `ignore-for-release`.

**Revisit when** a second regular contributor appears, or when incoming issues outnumber what one
person tracks by reading them — whichever comes first. At that point `graduate-backlog` records a
hybrid boundary and seeds the tracker properly. Doing it now would be building a queue for a
queue's sake.

**Alternatives rejected**:

| Alternative | Why not |
|---|---|
| Seed milestones and issues from M1–M5 now | Four of five describe finished work; the fifth is an upstream wait. A tracker of things nobody can act on trains people to ignore it |
| Enable a Projects board | A board is a view over a backlog. There is no backlog to view |
| Leave it undecided | The failure mode this gate exists to catch. An unstated position is indistinguishable from an oversight, and the next person cannot tell whether the empty tracker is deliberate |

## Consequences

**Positive**:

- A contributor arriving at an empty tracker finds a README that explains why, rather than
  inferring the project is abandoned.
- No duplicate source of truth to keep synchronised between the roadmap and a tracker.
- Incoming issues are visible as incoming, not lost among self-filed roadmap items.

**Negative**:

- Work in progress is invisible until it lands. With one maintainer that is not a collision risk,
  but it does mean a would-be contributor cannot see what is being worked on.
- "Where do I start?" has no `good first issue` to point at. Mitigated by
  [CONTRIBUTING.md](../../CONTRIBUTING.md) being explicit about scope and about what is
  deliberately excluded.
- If the project grows quickly the transition is a chunk of work rather than an increment.

## References

- [SPARK analysis](../inception/spark-analysis.md) — scope, out-of-scope decisions, open questions
- `graduate-backlog` (project-orchestration-skills) — the path to take when this is revisited
- [CONTRIBUTING.md](../../CONTRIBUTING.md)
