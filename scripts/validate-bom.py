#!/usr/bin/env python3
"""Validate a CycloneDX document against the official JSON Schema.

The Go test suite checks the BOM's structure and the claims it must carry, but not its
conformance to the specification — doing that in Go would mean taking on a JSON Schema
dependency for one check. This script closes that gap and is meant to be run in CI.

Usage:
    ansible-bom scan <root> > bom.json
    python3 scripts/validate-bom.py bom.json

Schemas are cached under .cache/cyclonedx/ after the first run, so repeat runs are offline.
"""

from __future__ import annotations

import json
import sys
import urllib.request
from pathlib import Path

SPEC_VERSION = "1.6"
BASE = "https://raw.githubusercontent.com/CycloneDX/specification/master/schema"
# spdx and jsf are referenced by the main schema and must be resolvable.
SCHEMAS = {
    f"bom-{SPEC_VERSION}.schema.json": None,
    "spdx.schema.json": "http://cyclonedx.org/schema/spdx.schema.json",
    "jsf-0.82.schema.json": "http://cyclonedx.org/schema/jsf-0.82.schema.json",
}
CACHE = Path(__file__).resolve().parent.parent / ".cache" / "cyclonedx"


def fetch(name: str) -> dict:
    CACHE.mkdir(parents=True, exist_ok=True)
    path = CACHE / name
    if not path.exists():
        with urllib.request.urlopen(f"{BASE}/{name}", timeout=30) as r:
            path.write_bytes(r.read())
    return json.loads(path.read_text())


def main(argv: list[str]) -> int:
    if len(argv) != 1:
        print(__doc__, file=sys.stderr)
        return 2

    try:
        from jsonschema import Draft7Validator, RefResolver
    except ImportError:
        print("jsonschema is required: pip install jsonschema", file=sys.stderr)
        return 2

    schema = fetch(f"bom-{SPEC_VERSION}.schema.json")
    store = {schema.get("$id", ""): schema}
    for name, uri in SCHEMAS.items():
        if uri:
            store[uri] = fetch(name)

    resolver = RefResolver.from_schema(schema, store=store)
    validator = Draft7Validator(schema, resolver=resolver)

    bom = json.loads(Path(argv[0]).read_text())
    errors = sorted(validator.iter_errors(bom), key=lambda e: list(e.path))

    if not errors:
        print(f"{argv[0]}: valid CycloneDX {SPEC_VERSION}")
        return 0

    print(f"{argv[0]}: {len(errors)} validation error(s)", file=sys.stderr)
    for e in errors:
        where = "/".join(map(str, e.path)) or "(root)"
        print(f"  {where}: {e.message}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
