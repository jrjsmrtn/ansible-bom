# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities **privately** — do not open a public issue.

- Preferred: [GitHub Security Advisories](https://github.com/jrjsmrtn/ansible-bom/security/advisories/new) (private)
- Or email: jrjsmrtn@gmail.com

We aim to acknowledge within 5 business days and to share a remediation timeline after triage. We
follow **coordinated disclosure**: please allow reasonable time for a fix before public disclosure.

This is a single-maintainer project. Response times reflect that, and saying so up front is more
useful than a service-level commitment that would not be met.

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | ✅ |
| < 0.2   | ❌ |

Pre-1.0, only the latest minor line receives fixes. See
[ADR-0004](docs/adr/0004-provisional-purl-identifiers.md) for why 1.0 is gated on an external
condition rather than on feature completeness.

## Scope

`ansible-bom` reads files that it does not control, and that is where its real attack surface is.

**In scope**

- **Parsing untrusted content.** The tool is pointed at an Ansible content tree and parses
  `MANIFEST.json`, `FILES.json`, `meta/main.yml`, `meta/.galaxy_install_info` and
  `requirements.yml` from it. Those files arrive from Galaxy, from git repositories, or from
  whoever wrote them. A crafted file that causes a crash, unbounded memory growth, a panic that
  cannot be recovered, or path traversal outside the scanned root is in scope.
- **Path handling.** File names inside `FILES.json` are attacker-influenced. Any route by which
  they cause a read outside the collection directory is in scope, including during `verify`.
- **Output integrity.** A crafted tree that causes the tool to emit a BOM asserting something
  false — a component reported as verified when it was not, checksummed content reported at the
  wrong assurance tier, or a document claiming completeness when content failed to parse — is a
  security issue, not merely a bug. Overstated assurance is the failure mode this project is
  built to avoid.
- **The release artifacts.** Compromise of the published binaries, checksums or SBOMs.

**Out of scope**

- **What `verify` cannot detect, by design.** It checks installed files against the checksums
  recorded at install time, following the chain `MANIFEST.json` → `FILES.json` → files. Nothing
  records `MANIFEST.json`'s own hash, so an adversary who rewrites the whole chain defeats the
  check. This is documented, tested, and a property of the Ansible ecosystem — detecting it needs
  the Galaxy server or a signature. Reports that the tool "fails to detect" this are not
  vulnerabilities.
- **Absence of vulnerability data.** No vulnerability database indexes Ansible collections or
  roles. The tool reports coverage status per component precisely so an empty result is not read
  as clean. A report that the tool "misses known CVEs in a collection" is a gap in the ecosystem,
  not in this tool.
- **Roles having no checksums.** They carry none anywhere. The tool reports them as
  *unverifiable*, never as verified.
- **The content being scanned.** Vulnerabilities in the Ansible collections or roles you point
  the tool at belong to their maintainers.

The tool performs no network I/O, requires no credentials, and writes only where you tell it to.
