// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package drift

import (
	"testing"

	"github.com/jrjsmrtn/ansible-bom/content"
	"github.com/jrjsmrtn/ansible-bom/internal/requirements"
)

func inventory(cs ...content.Component) content.Inventory {
	return content.Inventory{Components: cs}
}

func collection(name, version string) content.Component {
	return content.Component{
		Kind: content.KindCollection, Namespace: "ns", Name: name,
		Version: version, Origin: content.OriginGalaxy, Tier: content.TierChecksummed,
	}
}

func galaxyRole(name, version string) content.Component {
	return content.Component{
		Kind: content.KindRole, Namespace: "author", Name: name,
		Version: version, Origin: content.OriginGalaxy, Tier: content.TierNameVersionOnly,
	}
}

func localRole(name string) content.Component {
	return content.Component{
		Kind: content.KindRole, Name: name,
		Origin: content.OriginLocal, Tier: content.TierNameVersionOnly,
	}
}

func kinds(r Report) map[Kind]int { return r.Counts() }

func TestCompareFindings(t *testing.T) {
	inv := inventory(
		collection("declared_pinned", "1.0.0"),   // matches exactly
		collection("declared_mismatch", "2.0.0"), // declared 1.0.0
		collection("undeclared", "3.0.0"),        // transitive
	)
	req := requirements.File{Collections: []requirements.Declaration{
		{Kind: requirements.KindCollection, Name: "ns.declared_pinned", Version: "1.0.0"},
		{Kind: requirements.KindCollection, Name: "ns.declared_mismatch", Version: "1.0.0"},
		{Kind: requirements.KindCollection, Name: "ns.absent", Version: "1.0.0"},
	}}

	got := kinds(Compare(inv, req))
	want := map[Kind]int{
		KindVersionMismatch: 1, // declared_mismatch
		KindUndeclared:      1, // undeclared
		KindMissing:         1, // absent
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d (all: %v)", k, got[k], w, got)
		}
	}
	if got[KindUnpinned] != 0 {
		t.Errorf("unpinned = %d, want 0 — every declaration names an exact version", got[KindUnpinned])
	}
}

// The defect this guards against was found by running on a real control node: local roles are
// neither declared nor versioned, so a naive comparison reported all 48 of them twice and buried
// the eight findings that mattered.
func TestFirstPartyRolesAreNotDrift(t *testing.T) {
	inv := inventory(
		localRole("site_common"),
		localRole("site_database"),
		galaxyRole("from_galaxy", "1.0.0"),
	)
	req := requirements.File{Roles: []requirements.Declaration{
		{Kind: requirements.KindRole, Name: "author.from_galaxy", Version: "1.0.0"},
	}}

	got := kinds(Compare(inv, req))

	if got[KindFirstParty] != 2 {
		t.Errorf("first-party = %d, want 2", got[KindFirstParty])
	}
	if got[KindUndeclared] != 0 {
		t.Errorf("undeclared = %d, want 0 — first-party roles are not dependencies", got[KindUndeclared])
	}
	if got[KindUnpinnable] != 0 {
		t.Errorf("unpinnable = %d, want 0 — first-party roles travel with the repository", got[KindUnpinnable])
	}
}

// A Galaxy-installed role that nobody declared is genuine drift, unlike a local one.
func TestUndeclaredGalaxyRoleIsDrift(t *testing.T) {
	got := kinds(Compare(inventory(galaxyRole("surprise", "1.0.0")), requirements.File{}))
	if got[KindUndeclared] != 1 {
		t.Errorf("undeclared = %d, want 1", got[KindUndeclared])
	}
}

func TestMutableSourceOutranksUnpinned(t *testing.T) {
	req := requirements.File{Collections: []requirements.Declaration{
		{Kind: requirements.KindCollection, Name: "git+https://example.com/o/ns.widget.git"},
		{Kind: requirements.KindCollection, Name: "ns.plain"},
	}}
	rep := Compare(inventory(collection("widget", "0.1.0"), collection("plain", "1.0.0")), req)

	got := kinds(rep)
	if got[KindMutableSource] != 1 {
		t.Errorf("mutable-source = %d, want 1", got[KindMutableSource])
	}
	// A mutable declaration must not also be reported as merely unpinned: it is the worse
	// finding, and reporting both would double-count it.
	if got[KindUnpinned] != 1 {
		t.Errorf("unpinned = %d, want 1 (only ns.plain)", got[KindUnpinned])
	}
	// Worst first.
	if rep.Findings[0].Kind != KindMutableSource {
		t.Errorf("first finding = %s, want %s", rep.Findings[0].Kind, KindMutableSource)
	}
}

func TestReproducible(t *testing.T) {
	pinnedReq := requirements.File{Collections: []requirements.Declaration{
		{Kind: requirements.KindCollection, Name: "ns.good", Version: "1.0.0"},
	}}

	// Everything pinned and matching, plus first-party content: reproducible.
	rep := Compare(inventory(collection("good", "1.0.0"), localRole("site_common")), pinnedReq)
	if !rep.Reproducible() {
		t.Errorf("want reproducible; findings: %+v", rep.Findings)
	}

	// An unpinnable Galaxy component breaks it.
	rep = Compare(inventory(
		collection("good", "1.0.0"),
		content.Component{Kind: content.KindRole, Name: "orphan", Origin: content.OriginGalaxy},
	), pinnedReq)
	if rep.Reproducible() {
		t.Error("want not reproducible: a non-first-party component has no version")
	}

	// An unpinned declaration alone does not: the tree today is still describable.
	rep = Compare(inventory(collection("good", "1.0.0")), requirements.File{
		Collections: []requirements.Declaration{{Kind: requirements.KindCollection, Name: "ns.good"}},
	})
	if !rep.Reproducible() {
		t.Error("an unpinned declaration should not by itself make the node irreproducible")
	}
}

func TestCompareTotals(t *testing.T) {
	rep := Compare(inventory(collection("a", "1"), collection("b", "2")), requirements.File{
		Collections: []requirements.Declaration{{Name: "ns.a", Version: "1"}},
		Roles:       []requirements.Declaration{{Name: "x.y", Version: "1"}},
	})
	if rep.Declared != 2 {
		t.Errorf("Declared = %d, want 2", rep.Declared)
	}
	if rep.Installed != 2 {
		t.Errorf("Installed = %d, want 2", rep.Installed)
	}
}
