// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package purl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrjsmrtn/ansible-bom/content"
)

// This file pins what this tool emits against the vendored purl-spec#854 proposal.
//
// It is a DIVERGENCE RECORD, not a conformance pass. The tool does not currently conform, and
// these tests assert the divergence exactly, so that they fail when it changes — whether because
// the proposal moved or because the tool was made to conform. Either is a decision that belongs in
// ADR-0004, and neither should be discoverable only by someone re-reading the PR by hand. That is
// precisely how the divergence went unnoticed: ADR-0004 paraphrased the proposal in prose, the
// paraphrase was wrong, and nothing could contradict it.
//
// See testdata/purl-spec-854/PROVENANCE.md for what the snapshot is and how to refresh it.

// definition is the subset of the purl-type-definition schema this test reads.
type definition struct {
	Type        string `json:"type"`
	TypeName    string `json:"type_name"`
	Description string `json:"description"`

	NamespaceDefinition struct {
		Requirement   string `json:"requirement"`
		CaseSensitive bool   `json:"case_sensitive"`
	} `json:"namespace_definition"`

	NameDefinition struct {
		Requirement string `json:"requirement"`
	} `json:"name_definition"`

	VersionDefinition struct {
		Requirement string `json:"requirement"`
	} `json:"version_definition"`

	QualifiersDefinition []struct {
		Key string `json:"key"`
	} `json:"qualifiers_definition"`

	Examples []string `json:"examples"`
}

func loadDefinition(t *testing.T) definition {
	t.Helper()
	path := filepath.Join("testdata", "purl-spec-854", "ansible-definition.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the vendored proposal: %v", err)
	}
	var d definition
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return d
}

// parsed is a purl decomposed into the components the type definition constrains.
type parsed struct {
	Type       string
	Namespace  string // empty when the purl carries none
	Name       string
	Version    string
	Qualifiers []string // keys only, which is all the definition constrains
}

// parsePurl decomposes a purl per the universal grammar in ECMA-427:
//
//	pkg:type/namespace/name@version?qualifiers#subpath
//
// The grammar is universal; a type definition constrains which components are required, never the
// syntax. Only the parts this test asserts on are decoded.
func parsePurl(t *testing.T, s string) parsed {
	t.Helper()
	rest, ok := strings.CutPrefix(s, "pkg:")
	if !ok {
		t.Fatalf("purl %q does not start with the pkg: scheme", s)
	}
	rest, _, _ = strings.Cut(rest, "#") // discard subpath

	var p parsed
	var qualifiers string
	rest, qualifiers, _ = strings.Cut(rest, "?")
	for _, q := range strings.Split(qualifiers, "&") {
		if key, _, found := strings.Cut(q, "="); found {
			p.Qualifiers = append(p.Qualifiers, key)
		}
	}

	// The version follows the LAST @, so an @ inside a namespace cannot be mistaken for one.
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		p.Version = rest[i+1:]
		rest = rest[:i]
	}

	segments := strings.Split(rest, "/")
	if len(segments) < 2 {
		t.Fatalf("purl %q has no name segment", s)
	}
	p.Type = segments[0]
	p.Name = segments[len(segments)-1]
	p.Namespace = strings.Join(segments[1:len(segments)-1], "/")
	return p
}

// TestVendoredProposalUnchanged guards the snapshot itself. Every divergence assertion below is
// meaningful only while these hold; a refresh that changes them silently would turn the other
// tests into assertions about nothing.
func TestVendoredProposalUnchanged(t *testing.T) {
	d := loadDefinition(t)

	if d.Type != Type {
		t.Errorf("proposal declares type %q, this package emits %q", d.Type, Type)
	}
	if got := d.NamespaceDefinition.Requirement; got != "required" {
		t.Errorf("namespace requirement is %q, was \"required\" when the divergence was recorded —\n"+
			"the proposal has moved; re-read the PR and revisit ADR-0004", got)
	}
	if got := d.NameDefinition.Requirement; got != "required" {
		t.Errorf("name requirement is %q, want \"required\"", got)
	}
	if got := d.VersionDefinition.Requirement; got != "optional" {
		t.Errorf("version requirement is %q, want \"optional\"", got)
	}
	if len(d.Examples) == 0 {
		t.Fatal("the proposal carries no examples; the tests below assert against them")
	}
}

// TestProposalExamplesCarryANamespace validates the parser against upstream's own data before any
// of it is used to judge this tool. If the parser is wrong, the divergence findings are worthless.
func TestProposalExamplesCarryANamespace(t *testing.T) {
	d := loadDefinition(t)

	for _, ex := range d.Examples {
		p := parsePurl(t, ex)
		if p.Type != Type {
			t.Errorf("example %q has type %q, want %q", ex, p.Type, Type)
		}
		if p.Namespace == "" {
			t.Errorf("example %q parsed with no namespace — the parser is wrong, since the "+
				"proposal requires one", ex)
		}
		if strings.Contains(p.Name, ".") {
			t.Errorf("example %q has a dotted name %q — upstream splits namespace and name into "+
				"separate segments, so a dot here means the parser is wrong", ex, p.Name)
		}
	}
}

// TestDivergence_NamespaceIsFoldedIntoName records the substantive divergence.
//
// The proposal makes namespace a REQUIRED, separate purl component: pkg:ansible/cisco/aci@2.13.0.
// This tool folds it into the name: pkg:ansible/cisco.aci@2.13.0, emitting no namespace at all.
// That is not a variant spelling — it is non-conformance with a machine-readable constraint.
//
// ADR-0004 claimed the opposite ("following PR #854's proposal", "most likely to be a no-op").
// When this test fails, the tool has been changed to conform, or the proposal has moved. Update
// ADR-0004 rather than the expectation.
func TestDivergence_NamespaceIsFoldedIntoName(t *testing.T) {
	d := loadDefinition(t)

	c := content.Component{
		Kind:      content.KindCollection,
		Namespace: "cisco",
		Name:      "aci",
		Version:   "2.13.0",
	}
	got := For(c)
	p := parsePurl(t, got)

	if d.NamespaceDefinition.Requirement != "required" {
		t.Fatal("guarded by TestVendoredProposalUnchanged")
	}

	if p.Namespace != "" {
		t.Errorf("this tool now emits a namespace (%q) for %q.\n"+
			"The divergence recorded in ADR-0004 no longer holds — the tool may now conform.\n"+
			"Update ADR-0004 and turn this into a conformance assertion.", p.Namespace, got)
	}
	if p.Name != "cisco.aci" {
		t.Errorf("name segment is %q, want the dotted form %q that records the divergence", p.Name, "cisco.aci")
	}

	const conformant = "pkg:ansible/cisco/aci@2.13.0" // the proposal's own first example
	if got == conformant {
		t.Errorf("emitted %q, which now matches the proposal — see above", got)
	}
}

// TestDivergence_RoleQualifierIsNotInTheProposal records the second divergence.
//
// The proposal is titled "Ansible Collection" and defines exactly four qualifiers. Legacy roles
// are not addressed by it at all, so the kind=role discriminator this tool needs — Galaxy
// namespaces roles and collections alike, so "author.name@version" is otherwise ambiguous — is an
// extension, not an implementation of the proposal.
func TestDivergence_RoleQualifierIsNotInTheProposal(t *testing.T) {
	d := loadDefinition(t)

	defined := make(map[string]bool, len(d.QualifiersDefinition))
	for _, q := range d.QualifiersDefinition {
		defined[q.Key] = true
	}

	key, _, _ := strings.Cut(RoleQualifier, "=")
	if defined[key] {
		t.Errorf("the proposal now defines a %q qualifier; this tool's role discriminator is no "+
			"longer an extension. Revisit ADR-0004 and the upstream report.", key)
	}

	role := content.Component{
		Kind:      content.KindRole,
		Namespace: "jborean93",
		Name:      "win_openssh",
		Version:   "0.3.2",
	}
	p := parsePurl(t, For(role))
	if len(p.Qualifiers) != 1 || p.Qualifiers[0] != key {
		t.Errorf("role purl qualifiers = %v, want exactly [%q]", p.Qualifiers, key)
	}

	// The scope statement is what makes roles a gap rather than an oversight in the examples.
	scope := strings.ToLower(d.TypeName + " " + d.Description)
	if strings.Contains(scope, "role") {
		t.Errorf("the proposal's scope now mentions roles (%q / %q) — the gap reported upstream "+
			"may have been addressed", d.TypeName, d.Description)
	}
}

// TestDivergence_ContestedQualifiersAreNotEmitted pins the deliberate omission in ADR-0004:
// vcs_url is the actively contested qualifier, so nothing this tool emits depends on it.
func TestDivergence_ContestedQualifiersAreNotEmitted(t *testing.T) {
	components := []content.Component{
		{Kind: content.KindCollection, Namespace: "community", Name: "general", Version: "11.4.0"},
		{Kind: content.KindRole, Namespace: "jborean93", Name: "win_openssh", Version: "0.3.2"},
		{Kind: content.KindCollection, Namespace: "orphan", Name: "thing"},
	}
	for _, c := range components {
		p := parsePurl(t, For(c))
		for _, q := range p.Qualifiers {
			switch q {
			case "vcs_url", "repository_url", "download_url", "packaging":
				t.Errorf("%s carries qualifier %q; ADR-0004 keeps provenance qualifiers out of "+
					"identifiers while the proposal is contested", For(c), q)
			}
		}
	}
}
