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
// The tool CONFORMS to the proposal's identifier shape, and these tests are driven by the
// proposal's own examples rather than by expectations written out here — so they follow upstream
// rather than restating it. The target is a moving one: the type is not registered, the PR is open
// with changes requested, and conformance therefore has to be re-established on every refresh of
// the snapshot rather than assumed. That is the point of the tests.
//
// Two departures are asserted as deliberate rather than accidental: the ?kind=role qualifier,
// which the proposal has no equivalent for because it is scoped to collections, and the omission
// of a namespace when none was observed on disk.
//
// This file exists because ADR-0004 once paraphrased the proposal in prose, the paraphrase was
// wrong for months, and nothing could contradict it.
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
		t.Errorf("namespace requirement is %q, was \"required\" when conformance was established —\n"+
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

// TestConformance_ProposalExamplesRoundTrip is the core conformance assertion.
//
// For every example the proposal publishes, it decomposes the purl, rebuilds a component from the
// parts, and requires this tool to emit the same identifier back. Nothing about the expected shape
// is written here — the expectations come from upstream, so refreshing the snapshot re-tests
// conformance instead of re-asserting a stale reading of it.
//
// Qualifiers are compared separately: the proposal's examples carry provenance qualifiers this
// tool deliberately does not emit (see TestConformance_ContestedQualifiersAreNotEmitted), so only
// type, namespace, name and version participate in the round trip.
func TestConformance_ProposalExamplesRoundTrip(t *testing.T) {
	d := loadDefinition(t)

	for _, ex := range d.Examples {
		want := parsePurl(t, ex)

		// Every published example names a collection; roles are outside the proposal's scope.
		c := content.Component{
			Kind:      content.KindCollection,
			Namespace: want.Namespace,
			Name:      want.Name,
			Version:   want.Version,
		}
		got := parsePurl(t, For(c))

		if got.Type != want.Type || got.Namespace != want.Namespace ||
			got.Name != want.Name || got.Version != want.Version {
			t.Errorf("round trip of %q\n  got  type=%q namespace=%q name=%q version=%q\n"+
				"  want type=%q namespace=%q name=%q version=%q\n"+
				"This tool no longer agrees with the proposal. Either it regressed, or the "+
				"snapshot was refreshed and the proposal moved — decide which in ADR-0004.",
				ex, got.Type, got.Namespace, got.Name, got.Version,
				want.Type, want.Namespace, want.Name, want.Version)
		}
	}
}

// TestConformance_NamespaceIsASeparateComponent states the specific property that was wrong for
// months and is the reason this file exists: namespace is its own purl component, never folded
// into the name.
func TestConformance_NamespaceIsASeparateComponent(t *testing.T) {
	c := content.Component{
		Kind:      content.KindCollection,
		Namespace: "cisco",
		Name:      "aci",
		Version:   "2.13.0",
	}
	const want = "pkg:ansible/cisco/aci@2.13.0" // the proposal's own first example
	if got := For(c); got != want {
		t.Errorf("For(cisco.aci) = %q, want %q", got, want)
	}

	p := parsePurl(t, For(c))
	if strings.Contains(p.Name, ".") {
		t.Errorf("name segment %q contains a dot — the namespace has been folded back into the "+
			"name, which is the exact non-conformance ADR-0004 records having corrected", p.Name)
	}
}

// TestDeparture_NamespaceOmittedWhenNotObserved records a deliberate departure.
//
// The proposal marks namespace required because it models Galaxy-installed collections, which
// always have one. Content found on disk without install metadata — a locally-authored role under
// roles/ — genuinely has none. Emitting a namespace-less purl is knowingly incomplete; inventing
// one would fabricate identity, which this tool must never do.
func TestDeparture_NamespaceOmittedWhenNotObserved(t *testing.T) {
	d := loadDefinition(t)
	if d.NamespaceDefinition.Requirement != "required" {
		t.Skip("the proposal no longer requires a namespace; this departure is moot")
	}

	local := content.Component{Kind: content.KindRole, Name: "site_common"}
	got := For(local)
	if want := "pkg:ansible/site_common?kind=role"; got != want {
		t.Errorf("For(local role) = %q, want %q", got, want)
	}
	if p := parsePurl(t, got); p.Namespace != "" {
		t.Errorf("a namespace (%q) was emitted for content that has none on disk — identity must "+
			"never be invented to satisfy a schema", p.Namespace)
	}
}

// TestConformance_NormalisationLowercases checks the proposal's case rule, which it states in
// prose ("Must be lowercased") alongside case_sensitive: false.
func TestConformance_NormalisationLowercases(t *testing.T) {
	d := loadDefinition(t)
	if d.NamespaceDefinition.CaseSensitive {
		t.Fatal("the proposal now marks namespace case-sensitive; revisit normalisation")
	}

	c := content.Component{Kind: content.KindRole, Namespace: "MyOrg", Name: "MyRole", Version: "1.0.0"}
	if got, want := For(c), "pkg:ansible/myorg/myrole@1.0.0?kind=role"; got != want {
		t.Errorf("For(MyOrg.MyRole) = %q, want %q", got, want)
	}
}

// TestDeparture_RoleQualifierIsNotInTheProposal records the second deliberate departure.
//
// The proposal is titled "Ansible Collection" and defines exactly four qualifiers. Legacy roles
// are not addressed by it at all, so the kind=role discriminator this tool needs — Galaxy
// namespaces roles and collections alike, so "author.name@version" is otherwise ambiguous — is an
// extension, not an implementation of the proposal.
func TestDeparture_RoleQualifierIsNotInTheProposal(t *testing.T) {
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

// TestConformance_ContestedQualifiersAreNotEmitted pins the deliberate omission in ADR-0004:
// vcs_url is the actively contested qualifier, so nothing this tool emits depends on it.
func TestConformance_ContestedQualifiersAreNotEmitted(t *testing.T) {
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
