import json, os, sys
# syft records the resolved absolute scan path in the `file` component, so an SBOM would
# otherwise depend on where it was built. -trimpath removes build paths from the binary; this
# does the same for the document. It rewrites a path to its basename and nothing else.
for path in sys.argv[1:]:
    bom = json.load(open(path))
    changed = 0
    for c in bom.get("components", []):
        if c.get("type") == "file" and os.path.isabs(c.get("name", "")):
            c["name"] = os.path.basename(c["name"])
            changed += 1
        for loc in c.get("properties", []):
            if loc.get("name", "").endswith(":path") and os.path.isabs(loc.get("value", "")):
                loc["value"] = os.path.basename(loc["value"])
                changed += 1
    json.dump(bom, open(path, "w"), indent=2)
    print(f"{path}: normalised {changed} path(s)")
