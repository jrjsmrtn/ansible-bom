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
	"strings"

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
}

// entry covers every shape an entry may take in either section. Entries may also be bare
// strings, handled before decoding into this.
type entry struct {
	Name    string `yaml:"name"`
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
		for _, n := range seq {
			if d, ok := declaration(n, KindRole); ok {
				f.Roles = append(f.Roles, d)
			}
		}
		return f, nil
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
	for _, n := range doc.Roles {
		if d, ok := declaration(n, KindRole); ok {
			f.Roles = append(f.Roles, d)
		}
	}
	return f, nil
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
		// appear in the wild in either section.
		name := e.Name
		if name == "" {
			name = e.Src
		}
		if name == "" {
			return Declaration{}, false
		}
		return Declaration{
			Kind:    kind,
			Name:    name,
			Version: e.Version,
			Source:  firstNonEmpty(e.Source, e.Src),
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
// which catches scp-style remotes such as "git@host:path/r.git".
func (d Declaration) IsDerived() bool {
	if d.Kind == KindRole {
		return strings.Contains(d.Name, "://") || strings.Contains(d.Name, "@")
	}
	return d.isURL()
}

// isURL is the collection-side test, and is not ansible's. See FQN and issue #14.
func (d Declaration) isURL() bool {
	n := d.Name
	return strings.HasPrefix(n, "git+") ||
		strings.Contains(n, "://") ||
		strings.HasPrefix(n, "git@")
}

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
	return d.isURL() || d.Type == "git" || d.SCM == "git" || d.Type == "url" || d.Type == "file"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
