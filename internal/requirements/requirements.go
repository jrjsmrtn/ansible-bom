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

// FQN is the name the declared content will install under.
//
// Most declarations name content directly. Entries given as a git URL do not: the URL's last
// path segment is conventionally "<namespace>.<name>", which is what ansible-galaxy installs it
// as. That derivation is a convention rather than a guarantee, so IsDerived reports when it was
// used and callers can qualify what they claim.
func (d Declaration) FQN() string {
	if !d.isURL() {
		return d.Name
	}
	s := strings.TrimSuffix(strings.TrimSpace(d.Name), "/")
	// A git URL may carry ",<version>" — ansible's own separator.
	if i := strings.Index(s, ","); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// IsDerived reports whether FQN was inferred from a URL rather than declared outright.
func (d Declaration) IsDerived() bool { return d.isURL() }

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
