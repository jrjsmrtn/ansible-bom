# SPARK Analysis — Ansible Galaxy BOM

Date: 2026-07-30
Status: Complete — recommendation below
Project name: **`ansible-bom`** (decided 2026-07-31; see [Naming](#naming))

A tool that inventories installed Ansible content — collections **and** legacy roles — emits a
CycloneDX SBOM, produces the resolved lockfile the ecosystem does not provide, and reports drift
against `requirements.yml`.

---

## S — Stakeholders

Ansible is infrastructure-as-code used to build and configure production estates. Its content
supply chain is therefore in scope for the same obligations as any other dependency graph —
SBOM demands from customers, regulatory support-period duties, and internal audit — while having
markedly weaker tooling than language package ecosystems.

### Primary stakeholders

| Stakeholder | Role | Needs | Influence | Engagement |
|---|---|---|---|---|
| **Enterprise IaC / platform engineers** | build and operate Ansible control planes and pipelines | reproducible builds; know what is actually installed across controllers and execution environments | High | primary design target; CI-first workflows |
| **Packagers** | repackage Ansible content (RPM/deb, internal mirrors, OEM bundles) | authoritative component lists with versions, licences and hashes for what they redistribute | High | needs identity that survives repackaging — see the purl `packaging` qualifier |
| **SecOps / product security** | audit and attest what ships | machine-readable inventory for IaC, drift detection, evidence for compliance | High | consumes output through existing SBOM tooling, not a new tool |
| **OEM delivery teams** | use Ansible IaC to build and deliver OEM infrastructure to customers | must **produce xBOMs for what they deliver** — the Ansible content is part of the provenance chain of the delivered infrastructure, and is currently a hole in it | High | strongest compliance-driven pull; drives the SBOM output more than the lockfile |

### Secondary stakeholders

| Stakeholder | Interest | Impact | Communication |
|---|---|---|---|
| Homelab and small-team operators | reproducible controllers without enterprise tooling | benefit from the same output; tolerant early adopters | a convenient early-validation channel, not the design target |
| BOM consumers (Dependency-Track, grype, CycloneDX tooling) | ingest the output | decides whether output is *useful* or merely *valid* | conformance-test against real ingesters |
| syft / Anchore maintainers | own the cataloger contract if we upstream | gatekeepers for the widest distribution path | engage early with a cataloger proposal |
| purl-spec / Ecma TC54 TG2 | own the `ansible` purl type | identifiers stay provisional until resolved | track [PR #854](https://github.com/package-url/purl-spec/pull/854) |
| Ansible community / Galaxy | own the metadata formats read here | format changes break parsing | read-only dependency |

**Who could block or derail this**: syft maintainers, if the cataloger path is chosen and a
proposal stalls — mitigated by the library path, which needs nobody's approval. Standards bodies
can churn our identifiers but cannot block the tool.

---

## P — Problem Definition

### Problem statement

An Ansible control plane's real content set is undeclared, unpinned, unreproducible and
uninventoried. Ansible Galaxy has no lockfile, and no SBOM generator catalogs collections or
roles, so there is no artifact describing what a controller actually runs — and therefore no way
to attest it, diff it, or rebuild it.

### Current state — measured, not assumed

A production Ansible controller of moderate size was inventoried on 2026-07-30 as a reference
environment:

| Observation | Value |
|---|---|
| Collections declared in `requirements.yml` | 13 |
| Roles declared | 4 |
| **Declarations pinned to a version** | **0** |
| Collections installed but never declared (transitive) | ≥4 |
| Galaxy-installed roles present but never declared | ≥1 |
| Content installed from git sources with no ref (tracking a moving branch) | 2 collections |

Roughly a quarter of installed collections were selected by nobody, at versions nobody chose. Two
more track whatever a branch head happened to be. A rebuild today and a rebuild in six months
produce materially different control planes with no record of the difference — and nothing in the
ecosystem will tell you that happened.

#### The idempotency consequence

Reproducibility undersells this. Ansible's core promise is **idempotent convergence**: run the
playbook again, get the same state. That guarantee is about module behaviour against a host — it
silently assumes the modules themselves are unchanged.

With unpinned content that assumption does not hold. A run in January and a run in June execute a
byte-identical playbook through *different code*: module defaults change, behaviours are
deprecated, bugs are fixed and introduced. The playbook is the same; the convergence is not. The
idempotency guarantee is quietly voided at the one layer nobody inspects, and nothing in the
ecosystem reports it.

That has three practical consequences for the primary audience:

- **Check mode becomes untrustworthy** — a dry run predicts what *today's* module version would
  do, which is not what ran last quarter.
- **Observed host drift is ambiguous** — you cannot tell configuration drift from tooling drift
  without knowing which module version converged the host.
- **Incident forensics are impossible** — "what code ran on that host in March" has no answer.

A lockfile fixes the code that performs the convergence, which is what makes idempotency a claim
about the system rather than about a single run. This matters more to enterprise IaC teams than
any BOM does, and it is available immediately without waiting on any standard.

#### Why upstream will not fix it

The lockfile idea is not new and was not rejected on merit so much as abandoned with its vehicles.
"Lock file support" was raised against the **Galaxy project** — `ansible/galaxy-issues#165`, later
`ansible/galaxy#1358` — trackers that are now archived. Meanwhile **mazer**, Ansible's
experimental collection client, actually *implemented* it (`install --lockfile`, and `list
--lockfile`/`--frozen`) before the tool itself was abandoned in 2020.

So the feature has been built once, in a tool that no longer exists, and the requests live in
archived trackers. `ansible-core` ships no lockfile today, and there is no visible work towards
one. The community answer is a hand-rolled `requirements.yml` + generated-lockfile convention,
reinvented per organisation.

> **Correction to an earlier draft of this analysis**: it stated that a lockfile request was
> "closed as *not planned* in 2022". That came from a secondary source and could not be confirmed
> against any issue tracker. The history above is what the primary sources actually show.

### Desired future state

- Rebuilds are reproducible from a committed lockfile.
- Drift between declared and installed is a report, not a discovery during an incident.
- Ansible content appears in the same SBOM pipeline as every other dependency ecosystem.
- Content installed from mutable sources is visible as such.

### Scope boundaries

**In scope**

- Walk an installed content tree — `ansible_collections/` **and** roles — reading
  `MANIFEST.json`, `FILES.json`, `meta/main.yml` and `meta/.galaxy_install_info`.
- **Roles are first-class**, not a lesser sibling of collections, despite carrying materially
  weaker metadata (see Analysis).
- Emit CycloneDX with purl identity, file hashes where available, licences, and the dependency
  graph (collection → collection, role → role).
- Emit a resolved lockfile suitable for committing and reinstalling from.
- Report drift against `requirements.yml`: undeclared transitive content, declared-but-missing,
  version mismatches, unpinned declarations, mutable git sources.

**Not in scope**

- **Dependency resolution.** The tool observes what `ansible-galaxy` did. Reimplementing the
  resolver would be a correctness liability with no upside.
- **Installing anything.** Read-only against the filesystem.
- **Vulnerability matching.** There is no Ansible ecosystem in OSV (assumption — see K). The
  output will be correct; no scanner will match against it yet.
- **OBOM of managed hosts.** What playbooks *deploy* is a different and much harder document.

**Deferred**

- Collection signature verification (`ansible-galaxy collection verify`, GPG detached signatures)
  carried into the BOM as attestation data — relevant to certified/Automation Hub content.
- Execution Environment images as a first-class input alongside filesystem trees.
- Publishing BOMs via TEA once that specification settles.

### Success criteria

1. Produces a CycloneDX document that validates, and that Dependency-Track ingests with component
   identity intact.
2. Detects the undeclared transitive content and mutable git sources present in the reference
   environment without being told about them.
3. A lockfile it emits, reinstalled into a clean tree, yields an identical inventory.
4. Inventories collections and roles with a single invocation, and states plainly where role
   coverage is weaker than collection coverage rather than silently degrading.
5. Runs offline against an existing tree, with no Galaxy credentials and no network.
6. Requires no change to how content is currently installed.

---

## A — Analysis

### Existing solutions

| Solution | Pros | Cons | Why not sufficient |
|---|---|---|---|
| `ansible-galaxy collection list` | built in | human-formatted; no hashes, no graph, no machine format; roles handled separately | not a BOM, not a lockfile |
| `ansible-galaxy collection verify` | verifies installed files against the server copy | verification only; needs network; no inventory; collections only | complements, does not replace |
| Hand-rolled lockfile conventions | solves reproducibility | per-organisation reinvention; no BOM, no drift report, no hashes | covers one goal of four |
| **mazer** (archived) | actually implemented `install --lockfile` and `list --lockfile/--frozen` | abandoned in 2020; was a collection client, never covered roles or BOMs | proves the design is feasible; nothing inherited it |
| `syft` / `cdxgen` / `trivy` | mature; CycloneDX and SPDX output; already in enterprise CI | Ansible absent from syft's documented ecosystem list | **the gap — and the opportunity** |
| Execution Environments (`ansible-builder`) | pins content into an image | a container image is not an answer to "what is in it" | different problem; also a future input |

The gap is precise: **nothing emits CycloneDX with purl identity, hashes and a dependency graph
for Ansible content.**

### Metadata available — verified on disk

| Question | Collections | Roles |
|---|---|---|
| Structured manifest? | **Yes** — `MANIFEST.json` (namespace, name, version, dependencies, licence, repository) | Partial — `meta/main.yml`; Galaxy installs add `meta/.galaxy_install_info` (version, install date) |
| File checksums? | **Yes** — sha256 on every regular file (3290/3290 in one 4318-entry manifest) | **None.** No `FILES.json` equivalent exists |
| Same metadata for git-sourced installs? | **Yes** — `ansible-galaxy` builds the artifact from the checkout first, so on-disk layout is uniform | n/a |
| Version discoverable without parsing? | Yes — also encoded in sibling `<ns>.<name>-<version>.info` directories | Only from `.galaxy_install_info` |

Three findings that shape the design:

1. **Roles are integrity-blind.** Collections give sha256 for every file; roles give nothing. A
   role's "identity" is a version string in a dotfile written at install time. First-class role
   support therefore means an honest two-tier model, not a pretence of parity — and the tool must
   say which tier a component is in.
2. **Role install metadata is hostile to parse.** `.galaxy_install_info` records `install_date` as
   a *locale-formatted* string — observed values in a non-English locale with accented month
   abbreviations. Versions are inconsistently prefixed (`v0.3.2` alongside `3.5.0`), which matters
   for purl normalisation. Locally-authored roles have no `.galaxy_install_info` at all.
3. **Author-supplied manifest fields are untrustworthy.** A locally-authored collection in the
   reference tree carried the unedited Galaxy skeleton placeholder in its `repository` field. Only
   namespace, name and version are reliable for identity; everything else is informational.

### Is `ansible-galaxy` itself extensible?

Asked because the most natural home for this would be `ansible-galaxy lock` and
`ansible-galaxy bom` — shipping inside the tool the audience already runs, exactly the argument
that favours a syft cataloger for the BOM half.

**It is not.** Ansible's plugin architecture covers *playbook execution* — modules, lookup, filter,
callback, connection, strategy, inventory and so on — all loaded at runtime for running plays. The
`ansible-galaxy` CLI is a fixed set of subcommands implemented in `ansible-core`
(`lib/ansible/cli/galaxy.py`); there is no documented extension point, no entry-point mechanism,
and no plugin type for adding a subcommand. Nothing in the CLI documentation or the Galaxy
developer guide describes one.

*This is a negative-evidence conclusion* — absence of any documented hook, not a statement in the
docs that extension is unsupported. Worth a direct confirmation before it hardens into a design
assumption.

Two consequences:

1. **Lockfile and drift must ship as a standalone tool.** There is no "plug into `ansible-galaxy`"
   path, so the Ansible-adjacency advantage that would have favoured Python largely evaporates —
   a Python tool would be just as separate from `ansible-galaxy` as a Go binary, while being
   harder to distribute.
2. **The upstream path for the BOM half runs through syft, not Ansible.** syft has a cataloger
   interface and invites extension; `ansible-galaxy` has neither.

### Technology options

The decisive question is **distribution**, not language ergonomics. The primary audience already
runs SBOM tooling in CI; a tool they must separately adopt starts at a disadvantage.

| Option | Fit | Maturity | Decision |
|---|---|---|---|
| **Go — syft cataloger + standalone binary** | Highest. syft exposes a cataloger interface (`syft/cataloger`, `generic.Cataloger`, and a documented `create_custom_sbom` example), so Ansible coverage can land *inside the tool enterprises already run*. Upstreaming makes it free for every syft user; the library path ships immediately without waiting for anyone. Single static binary suits CI better than a Python environment that must coexist with `ansible-core`'s. SPDX output and grype integration come free. | High | **Selected** |
| **Python — standalone CLI** | Good. Installs next to `ansible-core` via pip/pipx; `cyclonedx-python-lib` is the reference implementation; natural home for the Ansible-workflow features. But it is another tool to adopt, gets no syft/grype integration, and pins the project to a runtime the target environments must already be managing carefully. | High | Rejected — see below |

**Selected: Go.** Rationale:

- The BOM capability belongs in syft. A cataloger reaches every existing user at zero adoption
  cost, which is worth more than any language preference.
- The library path de-risks upstream: ship a standalone binary embedding the cataloger, propose it
  upstream in parallel, and lose nothing if the proposal stalls.
- Lockfile and drift are **Ansible workflow** features that syft would never host — the standalone
  binary carries them. One language, one codebase, two delivery surfaces.
- Single-binary distribution matches enterprise CI, air-gapped mirrors and packager workflows
  better than a Python dependency.

The honest cost: Python would be faster to write, is closer to the Ansible ecosystem's own
language, and has the more mature CycloneDX library. That is a real trade accepted deliberately in
exchange for distribution reach.

### Constraints

- Must work offline against an existing tree; no Galaxy credentials.
- Must handle content installed from Galaxy, git, tarball and local paths.
- No standardised purl type yet (see Risks).
- Must not require changes to existing installation workflows.

### Dependencies

| Dependency | Type | Status | Risk if unavailable |
|---|---|---|---|
| syft cataloger interface | Technical | Available and documented | fall back to emitting CycloneDX directly |
| CycloneDX Go library | Technical | Available | hand-roll JSON; annoying, not blocking |
| purl `ansible` type (PR #854) | External standard | **Open, unmerged** | identifiers remain provisional |
| `MANIFEST.json` / `FILES.json` / `.galaxy_install_info` layouts | Technical | Stable in practice, unversioned contract | format change breaks parsing |

---

## R — Risk Assessment

| ID | Risk | Category | Likelihood | Impact | Score | Mitigation |
|---|---|---|---|---|---|---|
| R1 | No OSV coverage for Ansible → BOM enables no vulnerability matching, **and scanners report the gap as zero findings rather than as unknown** | Technical | **Confirmed** (POC-2) | M | **High** | Lead on idempotency, reproducibility, drift and attestation; declare coverage status per component ([ADR-0006](../adr/0006-declare-vulnerability-coverage-status.md)) |
| R2 | Roles cannot be given integrity data that does not exist | Technical | H (certain) | M | **High** | Explicit two-tier model; surface the gap in output rather than hiding it |
| R3 | syft upstream proposal stalls or is declined | External | M | M | Med | Library path ships regardless; upstreaming is an accelerator, not a dependency |
| R4 | purl `ansible` type changes or is rejected | External | M | M | Med | **Handled by release gating rather than mitigation** — 1.0 waits on the type being approved *and* implemented (see Synthesis). 0.x ships with provisional identifiers, constructed in one function, avoiding the contested `vcs_url` qualifier |
| R5 | Git-sourced content records no commit — identity is a `galaxy.yml` version with no link to source state | Technical | H (already true) | M | Med | Report mutable sources as findings; recommend pinned refs |
| R6 | Locale-formatted `install_date` and inconsistent version prefixes break parsing | Technical | H (already observed) | L | Med | Treat `install_date` as opaque; normalise versions defensively; fixtures from real trees |
| R7 | Enterprise users expect Execution Environments and Automation Hub on day one | Schedule | M | M | Med | Deferred explicitly; filesystem trees first, EE images as the next input |
| R8 | Go inexperience relative to Python slows early delivery | Resource | M | L | Low | Scope v1 tightly; the parsing work is simple |

### Top 3

1. **R1 — the vulnerability-matching gap.** The instinctive enterprise pitch for an SBOM is "so
   scanners find CVEs". That will not work here yet, and the disappointment would arrive after the
   work is done. The deliverables are **idempotency you can actually rely on**, reproducibility,
   drift, attestation-readiness and compliance evidence. Idempotency is the one that needs no
   qualification and no external standard — lead with it.
2. **R2 — role metadata poverty.** Roles are now first-class, but they carry no checksums and a
   version string written by the installer. Any BOM entry for a role is weaker than one for a
   collection, and the tool must be explicit about that rather than emitting components that look
   equivalent. Getting this wrong produces an SBOM that overstates its own assurance — worse than
   no SBOM.
3. **R3 — upstream acceptance.** The strategic value of the Go choice is syft reach. If that path
   closes, the standalone binary still works but the adoption advantage over the rejected Python
   option largely evaporates. *Mitigation*: engage syft maintainers with a cataloger proposal
   early, before the code shape is fixed.

---

## K — Knowledge Assessment

### What we know (verified 2026-07-30)

- `ansible-core` ships **no lockfile**. Requests were raised against the Galaxy project
  (`ansible/galaxy-issues#165`, `ansible/galaxy#1358`) in trackers now archived; **mazer**
  implemented the feature and was itself abandoned in 2020. No confirmed "closed as not planned"
  decision exists — the feature was orphaned, not refused.
- **`ansible-galaxy` is not extensible** — confirmed 2026-07-31 against `ansible-core` 2.20.0
  source. Subcommands are hardcoded `add_parser()` calls dispatching to bound
  `execute_<action>` methods; no plugin loader or entry-point machinery is imported; the package
  declares only `console_scripts`; and every plugin type under `ansible/plugins/` is a
  playbook-execution concern. The similarly-named `galaxy_server` "plugin type" is config-only —
  it names servers, it does not add behaviour. See
  [ADR-0003](../adr/0003-go-with-a-syft-cataloger-strategy.md).
- purl has **no `ansible` type**; PR #854 is open with one approval and one changes-requested, and
  live disagreement over `vcs_url` syntax and whether `packaging` should defer to rpm/deb types.
- Collections carry `MANIFEST.json` + `FILES.json` with **sha256 on every regular file**,
  uniformly for Galaxy- and git-sourced installs.
- Roles carry **no checksums**; Galaxy-installed roles carry `meta/.galaxy_install_info` with a
  version and a locale-formatted install date; locally-authored roles carry neither.
- Role-to-role dependencies exist in `meta/main.yml` but are rare in practice.
- Ansible does not appear in syft's documented ecosystem list; syft exposes a cataloger interface
  and a custom-cataloger example, making both the library and upstream paths viable.
- The reference environment has zero pinned declarations, ≥4 undeclared transitive collections,
  ≥1 undeclared Galaxy role, and 2 collections tracking mutable git sources.

### What we assume (needs validation)

| Assumption | Basis | How to validate | Priority |
|---|---|---|---|
| ~~OSV has no Ansible/Galaxy ecosystem~~ | — | — | **Confirmed 2026-07-31 — see POC-2 below** |
| Dependency-Track ingests components with an unregistered purl type | it accepts arbitrary purl strings | upload a sample BOM | High |
| A syft cataloger can emit components with a non-standard purl type | syft does not appear to constrain purl types | prototype against the library | High |
| Reinstalling from an emitted lockfile is reproducible | `ansible-galaxy` is deterministic given exact versions | install to a temp tree and compare | Medium |

### Knowledge gaps

| Gap | Impact if unfilled | Approach |
|---|---|---|
| syft maintainers' appetite for an Ansible cataloger | decides the strategic distribution path | **Proposal opened 2026-07-31: [anchore/syft#5129](https://github.com/anchore/syft/issues/5129).** Gap now blocked on their reply, not on our action |
| How tarball- and local-path-installed content differs on disk | parser misses a source type | install one of each into a scratch tree |
| Whether collection GPG signature data is retrievable post-install | blocks the deferred attestation work | read `ansible-galaxy collection verify` internals |
| Execution Environment image layout | shapes the deferred EE input | inspect a built EE |

### Proof-of-concept needs

| POC | Purpose | Success criteria | Effort |
|---|---|---|---|
| **POC-1 — parse a real tree** | validate against messy reality | accurate component list including transitive, git-sourced and role entries | Hours. *Largely done during this analysis; metadata questions resolved.* |
| **POC-2 — OSV coverage query** | settle the value proposition | definitive answer | **Done 2026-07-31 — see below** |
| **POC-3 — syft cataloger spike** | prove the strategic path | a custom cataloger emitting one Ansible component through syft | 1–2 days |
| POC-4 — Dependency-Track round-trip | confirm output is useful, not merely valid | components ingest with identity intact | Half a day |

### POC-2 result — OSV coverage (executed 2026-07-31)

**Confirmed: OSV has no Ansible ecosystem, and it fails silently.**

| Query | Result |
|---|---|
| `pkg:pypi/ansible-core@2.16.0` *(control)* | 12 advisories — the API call is correct |
| `pkg:ansible/community.general@11.4.0` | `{}` — **empty, not an error** |
| `{"name":"community.general","ecosystem":"Ansible"}` | `invalid ecosystem` |
| `{"name":"community.general","ecosystem":"Galaxy"}` | `invalid ecosystem` |
| OSV schema ecosystem list | zero occurrences of "ansible" or "galaxy" |

Two consequences, one of them unanticipated:

1. **The value framing holds.** No scanner will match Ansible collections or roles. Reproducibility,
   idempotency, drift and attestation are the deliverables; vulnerability matching is not
   available and should never be implied.
2. **The failure mode is worse than absence — it is silent.** An unregistered purl type returns an
   empty result set, indistinguishable from "queried and clean". A Dependency-Track dashboard
   showing *0 vulnerabilities* against Ansible components does not mean they are safe; it means
   nobody looked. Nothing in the pipeline says so.

This is the same class of hazard as the collection/role assurance gap: output that reads as
assurance when it means *unknown*. It is compounded by **mixed coverage within one document** —
a control node's Python components (`pkg:pypi/ansible-core`) *are* covered and will show real
findings, sitting in the same BOM beside collection components that are structurally incapable of
showing any. A document-level disclaimer cannot express that; it has to be per component.

Recorded as [ADR-0006](../adr/0006-declare-vulnerability-coverage-status.md).

---

## Synthesis

### Viability

| Dimension | Score (1–5) | Notes |
|---|---|---|
| Stakeholder alignment | 4 | clear compliance-driven pull from enterprise, packagers and secops; unvalidated by direct contact |
| Problem clarity | 5 | measured on a real controller; the numbers are unambiguous |
| Solution feasibility | 4 | collection metadata is richer than expected; role metadata is genuinely poor and caps what is achievable there |
| Risk manageability | 4 | main risks are expectation-setting and an upstream relationship, both addressable by decision and early engagement |
| Knowledge readiness | 4 | the big unknowns resolved during analysis; remaining gaps are days, not months |
| **Overall** | **4.2** | |

### Recommendation

**Proceed with conditions.**

The problem is real and measured rather than assumed. Ansible is infrastructure-as-code carrying
the same supply-chain obligations as any dependency graph, with markedly worse tooling: no
lockfile, orphaned rather than refused, and no cataloger in any mainstream SBOM generator. The
metadata needed is already on disk and, for collections, better than expected. The Go/syft path
converts the BOM half from "another tool to adopt" into "coverage appears in the tool they already
run".

The strongest argument is not the BOM at all. Unpinned content means the code performing
convergence changes underneath an unchanged playbook, which voids Ansible's central idempotency
guarantee in a way nothing currently reports. A lockfile fixes that, needs no standard, and pays
off on first use — the SBOM is what the compliance-driven stakeholders need, and the lockfile is
what makes the tool worth running daily.

### Release gating

Two independent gates, deliberately not conflated:

| Gate | Condition | Rationale |
|---|---|---|
| **Public release** | the tool is stable | the repository must be publishable from the first commit — no internal identifiers, hostnames or paths anywhere in history — but publication waits on stability, not on any standard |
| **v1.0** | the `ansible` purl type is **approved and implemented** | 1.0 is a compatibility promise. Committing to identifiers before the type is settled would mean either breaking them at 1.1 or carrying provisional ones forever |

Everything before that ships as **0.x with provisional identifiers**, clearly labelled as such in
the output. This turns the standards dependency from a risk into a schedule: 0.x delivers the
idempotency and drift value immediately, and 1.0 arrives when the ecosystem is ready for the
identity guarantee.

Note the external dependency this creates: [PR #854](https://github.com/package-url/purl-spec/pull/854)
is not under our control, currently has one approval and one changes-requested, and "implemented"
is a further step beyond "approved" — purl libraries and BOM consumers must recognise the type
before it is worth anything. 1.0 could be some way out.

### Naming

**Decided: `ansible-bom`.**

The question that blocked this was whether "BOM" undersells the lockfile, which is the day-one
value. It dissolves once the name is treated as a namespace rather than a pitch: the
**subcommands** carry the capabilities — `ansible-bom lock`, `ansible-bom drift`,
`ansible-bom scan`, `ansible-bom verify` — leaving the top-level name free to optimise for
discoverability.

Discoverability is worth optimising for here. Three of the four primary audiences (OEM delivery,
packagers, SecOps) arrive under compliance pressure and will search for "ansible sbom" or
"ansible bom". A single-maintainer project has no means to make a coined name findable.

Rejected alternatives:

| Candidate | Why not |
|---|---|
| `ansible-sbom` | Narrower than the tool — roles and collections, and the xBOM framing OEM needs. Forecloses scope for one extra letter of search traffic |
| `ansible-lock` | Hides the deliverable three of four primary audiences want, and implies the tool *performs* installation/locking, which is explicitly out of scope. Misleading about behaviour |
| `galaxy-bom` | Avoids the trademark but collides badly in search (Galaxy Project, Samsung) — which defeats the only reason to prefer a descriptive name |
| `ansible-content-bom` | Most accurate ("content" is the ecosystem's umbrella term for collections + roles) but too long for a CI-invoked CLI |
| `hangar`, `ephemeris` | Brandable and trademark-safe; `ephemeris` is genuinely apt for a lockfile. Both have zero discoverability. Held as fallbacks if the trademark position forces a change |

Consequences to manage:

- **Trademark.** "Ansible" is a Red Hat mark. The `ansible-` prefix is widely used by third-party
  tools, but Red Hat's policy discourages implying endorsement. Read the current policy before
  publishing and state non-affiliation prominently in the README. *Not yet verified.*
- **Search noise.** An unrelated `andrewrothstein/ansible-bom` repository exists.
- **No constraint on upstreaming.** If a cataloger lands in syft it takes syft's naming
  convention regardless — `ansible-collection-cataloger`, `ansible-role-cataloger`,
  `ansible-requirements-cataloger` — so the project name was chosen purely on distribution
  grounds.

### Conditions for success

1. Run **POC-2** (OSV coverage) before bootstrapping — minutes of work, and it confirms or refutes
   the framing.
2. **Engage syft maintainers early** with a cataloger proposal, before the code shape is fixed.
   The distribution advantage is the reason Go beat Python; validate it is available.
3. Commit to the **two-tier model** for collections versus roles, and make the asymmetry visible
   in the output.
4. Ship the **lockfile** capability first. It pays off on day one and depends on no unsettled
   standard.

### Immediate next steps

1. ~~POC-2: query OSV for a known collection.~~ **Done 2026-07-31** — assumption confirmed, plus
   the silent-zero finding now recorded as ADR-0006.
2. ~~Decide the final project name.~~ **Done** — `ansible-bom` (see [Naming](#naming)).
3. ~~Confirm directly that `ansible-galaxy` has no subcommand extension point.~~ **Done
   2026-07-31** — confirmed from `ansible-core` 2.20.0 source; the finding no longer rests on
   negative evidence.
4. `bootstrap-project` at **t1**, category Development, CLI tool, Go. Distribution profile
   **Private initially, Public once stable** — keep the repository publishable from the first
   commit: no internal identifiers, hostnames or paths anywhere in history.
5. First project ADRs: provisional `pkg:ansible` purl construction and the 1.0 gate (R4); the
   collection/role two-tier model (R2); Go and the syft-cataloger strategy.
6. ~~Open the syft proposal issue.~~ **Done 2026-07-31** — [anchore/syft#5129](https://github.com/anchore/syft/issues/5129). Awaiting maintainer response on two output-contract questions: how syft handles an ecosystem whose purl type is proposed but unmerged, and whether it can express "no integrity data exists" as distinct from "not collected".

### Open questions

1. ~~Final name.~~ Closed 2026-07-31 — see [Naming](#naming).
2. Licence, given the intended public release and a possible syft upstream contribution — an
   upstreamed cataloger must be compatible with syft's own licence.
3. Does v1 target filesystem trees only, or are Execution Environment images needed to be credible
   with the enterprise audience?
4. Red Hat trademark policy position on the `ansible-` prefix — a check, not a known problem, but
   it is the one thing that would force the name back open.

---

_Created from SPARK analysis on 2026-07-30. Measurements were taken from a real Ansible controller
used as a reference environment; no host, path, identifier or credential detail from that
environment is reproduced here._
