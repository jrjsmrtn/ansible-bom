# Parser fixtures

`MANIFEST.json` and `FILES.json` have no JSON Schema anywhere — their structure exists only as a
dict literal in `ansible-core`'s `_build_manifest()`. These fixtures are therefore the closest
thing to a specification for those formats that exists, which makes them load-bearing rather than
convenient. See [ADR-0007](../../../docs/adr/0007-schema-anchor-authored-files-fixture-anchor-generated-ones.md).

Do not "tidy" these files. They are shaped like reality, including the parts of reality that are
ugly.

## Provenance

| Fixture | Origin |
|---|---|
| `collection-galaxy/` | `community.general` 11.4.0, captured from a real installed tree. `FILES.json` trimmed to 6 file entries and 2 directory entries; `MANIFEST.json` verbatim |
| `collection-with-deps/` | `community.windows` 3.0.1, same treatment. Declares `ansible.windows: ">=3.0.0,<4.0.0"` — a real range constraint, not a synthetic one |
| `collection-future-format/` | Derived: `collection-galaxy` with `format` bumped to 2, to exercise the version gate |
| `collection-no-files/` | Derived: a `MANIFEST.json` with no `FILES.json` beside it — a shape `ansible-galaxy` does not produce, which must be reported rather than silently treated as a collection without checksums |
| `collection-placeholder-repo/` | Synthetic, modelled on a real observation: a collection shipping the unedited Galaxy skeleton placeholder (`https://www.github.com/my_org/my_collection`) in `repository`. Uses reserved documentation values throughout |
| `role-galaxy/` | Modelled on a real Galaxy-installed role: `v`-prefixed version and a locale-formatted `install_date` (`Dim 10 oct ...`), both observed in a real tree |
| `role-local/` | A role present in a repository with no install metadata and no namespace in its directory name |
| `role-with-deps/` | Exercises every dependency shape role metadata permits: bare string, `role:` mapping, `name:` mapping, `src:` mapping, plus a `collections:` list |
| `role-yaml-ext/` | `meta/main.yaml` rather than `meta/main.yml` |
| `not-content/` | Neither a collection nor a role |

## Rules

- Captured fixtures keep upstream bytes where practical; trimming is limited to reducing
  `FILES.json` volume, and the entry structure is preserved exactly.
- Synthetic fixtures use reserved-for-documentation values only (RFC 2606 domains, `example.*`
  names). No real hostnames, paths, or identifiers — this repository is published (ADR-0002 §7).
- `.editorconfig` exempts `testdata/**` from whitespace normalisation so captured bytes survive.
