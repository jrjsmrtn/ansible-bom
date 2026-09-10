// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package requirements parses requirements.yml — the file describing what a control node is
// *supposed* to have, as distinct from what it does have.
//
// Parsing is permissive by design. This tool inventories and compares; it does not lint. A file
// with unexpected keys, or shapes no schema anticipates, must still be read — ansible-lint already
// occupies the validation role (ADR-0007).
package requirements

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Kind distinguishes the two sections of a requirements file.
type Kind string

const (
	KindCollection Kind = "collection"
	KindRole       Kind = "role"
)

// Declaration is one requested collection or role.
type Declaration struct {
	Kind Kind

	// Name is the identity as declared. For entries given as a git URL this is the URL, and
	// FQN() derives the name the content will install under.
	Name string

	// Version is the declared constraint, verbatim. Empty means unpinned — the declaration
	// accepts whatever the server offers today.
	Version string

	// Source and Type are recorded where given. Type is "git", "file", "url", "galaxy", …
	Source string
	Type   string

	// SCM is the roles-section equivalent of Type.
	SCM string
}

// File is a parsed requirements.yml.
type File struct {
	Path        string
	Collections []Declaration
	Roles       []Declaration

	// Unread is content the file references that this tool did not read. It is recorded rather
	// than dropped: a declaration set that is silently short reports installed content as
	// undeclared, which is a wrong answer dressed as a finding (issue #15).
	Unread []Unread
}

// Unread is a reference this tool did not follow.
type Unread struct {
	Ref    string
	Reason string
}

// entry covers every shape an entry may take in either section. Entries may also be bare
// strings, handled before decoding into this.
type entry struct {
	Name string `yaml:"name"`
	// Role is the old-style alias for Name. ansible's RoleRequirement.role_yaml_parse rewrites
	// it to `name`, and it is in VALID_SPEC_KEYS alongside the rest.
	Role    string `yaml:"role"`
	Src     string `yaml:"src"`
	Version string `yaml:"version"`
	Source  string `yaml:"source"`
	Type    string `yaml:"type"`
	SCM     string `yaml:"scm"`
}

// document is the modern two-section form. The legacy form is a bare list of roles, handled
// separately in Parse.
type document struct {
	Collections []yaml.Node `yaml:"collections"`
	Roles       []yaml.Node `yaml:"roles"`
}

// Parse reads a requirements.yml.
func Parse(path string) (File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := ParseBytes(raw)
	f.Path = path
	return f, err
}

// ParseBytes parses requirements.yml content.
func ParseBytes(raw []byte) (File, error) {
	var f File

	// The legacy form is a bare sequence of roles, with no section keys at all.
	var seq []yaml.Node
	if err := yaml.Unmarshal(raw, &seq); err == nil && len(seq) > 0 {
		f.parseRoleEntries(seq)
		return f, nil
	}

	// A file ansible refuses is not a declaration set to compare against — a partial answer
	// derived from it would be worse than none (issue #15).
	if err := rejectWhatAnsibleRejects(raw); err != nil {
		return f, err
	}

	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return f, fmt.Errorf("parsing requirements: %w", err)
	}
	for _, n := range doc.Collections {
		if d, ok := declaration(n, KindCollection); ok {
			f.Collections = append(f.Collections, d)
		}
	}
	f.parseRoleEntries(doc.Roles)
	return f, nil
}

// parseRoleEntries reads the role entries of either form.
//
// One function for both, mirroring ansible: _parse_requirements_file calls parse_role_req for the
// legacy bare list AND for the v2 roles: section. Two hand-written loops here is precisely what
// let the include check cover one form and not the other (issue #24), so the fix is to remove the
// duplication rather than to add a second copy of the check.
//
// Collections deliberately have no equivalent: _init_coll_req_dict treats a non-dict entry as a
// name and Requirement.from_requirement_dict has no include handling, so `include:` is roles-only.
func (f *File) parseRoleEntries(nodes []yaml.Node) {
	for _, n := range nodes {
		if inc, ok := includeRef(n); ok {
			// ansible follows this and installs what it finds. Following it here means resolving
			// relative paths and detecting cycles (issue #26); saying the content was not read
			// costs nothing and removes the silent gap.
			f.Unread = append(f.Unread, Unread{
				Ref:    inc,
				Reason: "include: is not followed; roles declared there are not compared",
			})
			continue
		}
		if d, ok := declaration(n, KindRole); ok {
			f.Roles = append(f.Roles, d)
		}
	}
}

// rejectWhatAnsibleRejects mirrors the two file-level faults GalaxyCLI._parse_requirements_file
// raises on. Both mean ansible-galaxy will not install from this file at all, so comparing a
// control node against it answers a question nobody asked.
//
// Entry-level tolerance is unaffected: unknown keys ON AN ENTRY stay ignored, per ADR-0007.
// ansible is itself asymmetric here — it silently drops unknown role-entry keys while treating an
// unknown top-level key as fatal.
func rejectWhatAnsibleRejects(raw []byte) error {
	var top map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &top); err != nil {
		// Not a mapping; the caller's decode reports the shape error.
		return nil
	}
	if len(top) == 0 {
		// "No requirements found in file '%s'" upstream.
		return fmt.Errorf("no requirements found: the file declares neither roles nor collections")
	}
	var extra []string
	for k := range top {
		if k != "roles" && k != "collections" {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		// "Expecting only 'roles' and/or 'collections' as base keys" upstream. A typo'd section
		// name lands here, and every component it declares would otherwise read as undeclared.
		return fmt.Errorf("unknown top-level key(s) %s: ansible-galaxy accepts only 'roles' and 'collections', and refuses the file otherwise",
			strings.Join(extra, ", "))
	}
	return nil
}

// includeRef reports the target of a legacy `include:` entry.
func includeRef(n yaml.Node) (string, bool) {
	if n.Kind != yaml.MappingNode {
		return "", false
	}
	var e struct {
		Include string `yaml:"include"`
	}
	if err := n.Decode(&e); err != nil || e.Include == "" {
		return "", false
	}
	return e.Include, true
}

func declaration(n yaml.Node, kind Kind) (Declaration, bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value == "" {
			return Declaration{}, false
		}
		if kind == KindRole {
			return bareRole(n.Value)
		}
		return Declaration{Kind: kind, Name: n.Value}, true
	case yaml.MappingNode:
		var e entry
		if err := n.Decode(&e); err != nil {
			return Declaration{}, false
		}
		// The roles section names its identity field `src`; collections use `name`. Both
		// appear in the wild in either section. `role` is the old-style alias ansible rewrites
		// to `name` (issue #15) and comes last, matching that precedence.
		name := firstNonEmpty(e.Name, e.Src, e.Role)
		if name == "" {
			return Declaration{}, false
		}
		source := firstNonEmpty(e.Source, e.Src)
		if kind == KindCollection && !IsFQCN(name) {
			// ansible does exactly this: Requirement.from_requirement_dict sets req_name to None
			// when the name is not a valid FQCN, and identity comes from the fetched artefact's
			// galaxy.yml or MANIFEST.json instead. This tool never fetches, so it has no identity
			// to record — and a derived one would be a guess presented as a fact (issue #14).
			source = firstNonEmpty(e.Source, e.Src, name)
			name = ""
		}
		return Declaration{
			Kind:    kind,
			Name:    name,
			Version: e.Version,
			Source:  source,
			Type:    e.Type,
			SCM:     e.SCM,
		}, true
	}
	return Declaration{}, false
}

// bareRole parses a role given as a bare string, where ansible's comma form applies:
//
//	role_name[,version[,name]]
//
// This is the ONLY path on which the comma is a separator. RoleRequirement.role_yaml_parse
// splits it for a string entry and not for a mapping `src`, despite a comment in ansible itself
// claiming otherwise (`# New style: { src: 'galaxy.role,version,name' }`). A comma in a mapping
// `src` is passed through whole and resolves to nonsense — verified against ansible-core 2.20.0.
//
// More than two commas is an AnsibleError there. This tool inventories rather than lints, so it
// records the entry unsplit instead of failing the whole file; the name will not match anything
// installed, which is the honest outcome for a declaration ansible would reject.
func bareRole(v string) (Declaration, bool) {
	d := Declaration{Kind: KindRole, Name: v}
	if strings.Count(v, ",") > 2 {
		// AnsibleError there: "Invalid role line (%s). Proper format is
		// 'role_name[,version[,name]]'". Recorded unsplit rather than failing the file, so the
		// name matches nothing installed — the honest outcome for an entry ansible rejects.
		return d, true
	}
	switch parts := strings.SplitN(v, ",", 3); len(parts) {
	case 2:
		// src,version
		d.Source, d.Version = parts[0], parts[1]
		d.Name = parts[0]
	case 3:
		// src,version,name — the third field is what it installs AS, so it is the identity,
		// matching the mapping path where `name` wins over `src`.
		d.Source, d.Version, d.Name = parts[0], parts[1], parts[2]
	}
	return d, true
}

// FQN is the name the declared content will install under.
//
// For ROLES this ports ansible's RoleRequirement.repo_url_to_role_name faithfully, quirks
// included, because drift compares against what ansible actually installed — reproducing its
// surprises is the point, not a bug to smooth over. See ADR-0007 and issue #13.
//
// For COLLECTIONS ansible derives nothing. When a collection's `name` is not a valid FQCN,
// Requirement.from_requirement_dict sets it to None and the identity comes from the fetched
// artefact's own galaxy.yml or MANIFEST.json — which this tool never fetches. The derivation
// below is therefore ours, not ansible's, and is tracked as issue #14. It is left in place so
// this change stays scoped to roles; do not read it as upstream behaviour.
func (d Declaration) FQN() string {
	if d.Kind == KindRole {
		return repoURLToRoleName(d.Name)
	}
	if !d.isURL() {
		return d.Name
	}
	// NOT ansible's algorithm — see the note above and issue #14.
	s := strings.TrimSuffix(strings.TrimSpace(d.Name), "/")
	if i := strings.Index(s, ","); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// pythonKeywords is keyword.kwlist from CPython 3.14, which ansible's is_valid_collection_name
// consults via keyword.iskeyword().
//
// ⚠ This list is a property of the PYTHON RUNNING ANSIBLE, not of the collection format. ansible's
// own source says so: "NOTE: keywords and identifiers are different in different Pythons". A name
// valid under one interpreter can be invalid under another — `async` and `await` became keywords
// in 3.7. Soft keywords (`match`, `case`, `type`, `_`) are deliberately absent: keyword.iskeyword
// returns false for them, so they are legal identifiers.
var pythonKeywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "async": true, "await": true, "break": true, "class": true,
	"continue": true, "def": true, "del": true, "elif": true, "else": true,
	"except": true, "finally": true, "for": true, "from": true, "global": true,
	"if": true, "import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true,
	"return": true, "try": true, "while": true, "with": true, "yield": true,
}

// IsFQCN reports whether a string is a well-formed collection name.
//
// Mirrors AnsibleCollectionRef.is_valid_collection_name (ansible-core 2.20.0): exactly one dot,
// and each half a Python identifier that is not a Python keyword.
//
//	if collection_name.count(u'.') != 1:
//	    return False
//	return all(
//	    not iskeyword(ns_or_name) and ns_or_name.isidentifier()
//	    for ns_or_name in collection_name.split(u'.')
//	)
//
// Differential-tested against the running interpreter's own function; see the test for the
// corpus and the remaining divergences.
func IsFQCN(s string) bool {
	ns, name, found := strings.Cut(s, ".")
	if !found || strings.Contains(name, ".") {
		return false
	}
	return isIdentifier(ns) && isIdentifier(name)
}

// isIdentifier mirrors Python's str.isidentifier(): the first rune is ID_Start or "_", and every
// later rune is ID_Continue.
//
// ⚠ The Unicode half cannot be made exact, and it is worth knowing why before anyone tries.
// Sweeping all 0x110000 code points against the running interpreter found 4666 disagreements on
// ID_Start and 4718 on ID_Continue, from two causes:
//
//  1. Python tests XID_Start/XID_Continue, the NFKC-closed variants. Go's stdlib ships no XID
//     tables, so this uses ID_Start/ID_Continue — a strict superset. U+037A, U+309B and U+309C
//     are the everyday examples.
//  2. The two runtimes ship different Unicode versions: Go 17.0.0 here against Python 16.0.0.
//     That difference is not portable, not stable, and flips whenever either side updates. No
//     amount of care in this function removes it.
//
// It does not matter in practice, and ansible says why: its own error states a collection name
// must "contain characters from [a-zA-Z0-9_] only". The DOCUMENTED contract is ASCII while the
// implementation happens to be Unicode-permissive, so a name that lands in the gap cannot be a
// real Galaxy name. The keyword clause below, by contrast, IS exact — and it is the half that
// misfires on names a person might plausibly write.
func isIdentifier(s string) bool {
	if s == "" || pythonKeywords[s] {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isIDStart(r) {
				return false
			}
			continue
		}
		if !isIDContinue(r) {
			return false
		}
	}
	return true
}

func isIDStart(r rune) bool {
	if r == '_' {
		return true
	}
	if unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space) {
		return false
	}
	return unicode.In(r, unicode.L, unicode.Nl, unicode.Other_ID_Start)
}

func isIDContinue(r rune) bool {
	if isIDStart(r) {
		return true
	}
	if unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space) {
		return false
	}
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
}

// Identifiable reports whether the declaration names content this tool can match against an
// installed tree. It is false only for a collection whose name is not an FQCN, where ansible
// resolves identity by fetching the artefact and reading its manifest.
//
// A caller must branch on this rather than comparing FQN(), which is empty for such an entry:
// treating "" as a name makes every unidentifiable declaration collide with every other.
func (d Declaration) Identifiable() bool { return d.FQN() != "" }

// repoURLToRoleName mirrors ansible's RoleRequirement.repo_url_to_role_name
// (playbook/role/requirement.py:49, ansible-core 2.20.0):
//
//	if '://' not in repo_url and '@' not in repo_url:
//	    return repo_url
//	trailing_path = repo_url.split('/')[-1]
//	if trailing_path.endswith('.git'):     trailing_path = trailing_path[:-4]
//	if trailing_path.endswith('.tar.gz'):  trailing_path = trailing_path[:-7]
//	if ',' in trailing_path:               trailing_path = trailing_path.split(',')[0]
//	return trailing_path
//
// The ORDER is load-bearing and produces two results that look like bugs and are not:
//
//   - ".../r.git,v1.2.3" yields "r.git". The .git is not terminal when that check runs, so it
//     survives, and only the comma suffix is removed.
//   - ".../repo/" yields "". There is no trailing-slash trim, so the last segment is empty.
//
// Both are reproduced deliberately. A name this tool derives differently from ansible is a name
// that will not match what is installed, which is worse than an odd-looking one that does.
func repoURLToRoleName(name string) string {
	if !strings.Contains(name, "://") && !strings.Contains(name, "@") {
		return name
	}
	trailing := name
	if i := strings.LastIndex(trailing, "/"); i >= 0 {
		trailing = trailing[i+1:]
	}
	trailing = strings.TrimSuffix(trailing, ".git")
	trailing = strings.TrimSuffix(trailing, ".tar.gz")
	if i := strings.Index(trailing, ","); i >= 0 {
		trailing = trailing[:i]
	}
	return trailing
}

// IsDerived reports whether FQN was inferred rather than declared outright.
//
// For roles this is ansible's own test — a string containing "://" or "@" is treated as a URL,
// which catches scp-style remotes such as "git@host:path/r.git". For collections it is always
// false; see the body.
func (d Declaration) IsDerived() bool {
	if d.Kind == KindRole {
		return strings.Contains(d.Name, "://") || strings.Contains(d.Name, "@")
	}
	// Never true for a collection since issue #14: it either declares a valid FQCN outright, or
	// it has no name at all and Identifiable() reports that. Nothing is inferred either way.
	return false
}

// isURL is the collection-side test, and is not ansible's. See FQN and issue #14.
//
// It reads ref() rather than Name, because a collection declared by URL now carries that URL in
// Source with Name empty. Reading Name alone made Mutable() return false for exactly the
// declarations most likely to be mutable.
func (d Declaration) isURL() bool {
	n := d.ref()
	return strings.HasPrefix(n, "git+") ||
		strings.Contains(n, "://") ||
		strings.HasPrefix(n, "git@")
}

// ref is what the declaration points at: its name where it has one, otherwise its source. An
// unidentifiable collection has no name, and every shape test on this type wants the string the
// user actually wrote.
func (d Declaration) ref() string { return firstNonEmpty(d.Name, d.Source) }

// Pinned reports whether the declaration names one exact version.
//
// A range or a wildcard is not a pin: ">=1.0.0" and "*" both accept whatever the server offers
// at install time, which is the condition this tool exists to surface.
func (d Declaration) Pinned() bool {
	v := strings.TrimSpace(d.Version)
	if v == "" || v == "*" {
		return false
	}
	return !strings.ContainsAny(v, "><=~^,|*")
}

// Mutable reports whether the declaration resolves to a moving target: a source-control or
// URL source with no version or ref, which installs whatever the branch head is that day.
func (d Declaration) Mutable() bool {
	if strings.TrimSpace(d.Version) != "" {
		return false
	}
	return d.isURL() || d.Type == "git" || d.SCM == "git" || d.Type == "url" || d.Type == "file" ||
		d.Type == "dir" || d.Type == "subdirs"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
