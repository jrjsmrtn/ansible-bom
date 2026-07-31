#!/usr/bin/env python3
"""Print one version's section from a Keep a Changelog file.

Used to build GitHub release notes. The obvious alternative — the git tag annotation — is worse
in two ways that only show up once published:

  - a signed tag's annotation carries the PGP signature block, and the GitHub API returns it as
    part of the message, so it lands verbatim in the release body;
  - an annotation is plain text, and GitHub renders release notes as Markdown, so hard-wrapped
    prose reflows into run-on paragraphs.

The changelog is already Markdown, already reviewed, and is the single place a release is
described. Usage:

    python3 scripts/changelog-section.py 0.1.1 [CHANGELOG.md]
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


def section(text: str, version: str) -> str:
    version = version.lstrip("vV")
    lines = text.splitlines()

    start = None
    heading = re.compile(rf"^##\s+\[{re.escape(version)}\]")
    for i, line in enumerate(lines):
        if heading.match(line):
            start = i + 1
            break
    if start is None:
        raise SystemExit(f"no section for version {version} in the changelog")

    body: list[str] = []
    for line in lines[start:]:
        # Stop at the next version heading or at the link-reference block.
        if line.startswith("## ") or re.match(r"^\[[^\]]+\]:\s", line):
            break
        body.append(line)

    return "\n".join(body).strip("\n")


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__, file=sys.stderr)
        return 2
    version = argv[0]
    path = Path(argv[1]) if len(argv) > 1 else Path("CHANGELOG.md")
    out = section(path.read_text(encoding="utf-8"), version)
    if not out:
        raise SystemExit(f"section for {version} is empty")
    print(out)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
