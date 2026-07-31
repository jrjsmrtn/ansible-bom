// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package drift compares what a control node has against what its requirements.yml asks for.
//
// The gap between the two is where reproducibility is lost. Neither file alone shows it:
// requirements.yml records intent, the installed tree records outcome, and nothing in the
// ecosystem reports the difference.
package drift

import (
	"sort"

	"github.com/jrjsmrtn/ansible-bom/content"
	"github.com/jrjsmrtn/ansible-bom/internal/requirements"
)

// Kind classifies a finding.
type Kind string

const (
	// KindUndeclared: installed but named nowhere in requirements.yml — usually pulled in as a
	// transitive dependency, at a version nobody chose.
	KindUndeclared Kind = "undeclared"
	// KindMissing: declared but not installed.
	KindMissing Kind = "missing"
	// KindVersionMismatch: declared one exact version, a different one is installed.
	KindVersionMismatch Kind = "version-mismatch"
	// KindUnpinned: declared with no exact version, so a reinstall may produce something else.
	KindUnpinned Kind = "unpinned"
	// KindMutableSource: declared from git or a URL with no ref, so it tracks a moving target.
	KindMutableSource Kind = "mutable-source"
	// KindUnpinnable: installed with no version recorded anywhere, so it cannot be reproduced.
	KindUnpinnable Kind = "unpinnable"
	// KindFirstParty: content authored in this repository rather than installed from anywhere.
	// Informational, not drift — see firstParty.
	KindFirstParty Kind = "first-party"
)

// firstParty reports whether a component is authored here rather than installed from elsewhere.
//
// Local roles carry no version and appear in no requirements.yml, which naively makes them both
// "unpinnable" and "undeclared". On a real control node that produced 102 findings of which about
// eight mattered — the tool reproducing the alert fatigue it exists to prevent.
//
// They are neither: first-party roles are versioned by the repository that contains them, and
// declaring your own roles as dependencies of yourself would be wrong. They are reported once,
// as context, and excluded from the drift categories.
func firstParty(c content.Component) bool {
	return c.Kind == content.KindRole && c.Origin == content.OriginLocal && !c.Versioned()
}

// Finding is one difference between intent and reality.
type Finding struct {
	Kind      Kind
	Component string
	KindOf    content.Kind // collection or role
	Declared  string
	Installed string
	Detail    string
}

// Report is the result of a comparison.
type Report struct {
	Findings []Finding

	// Declared and Installed are totals, reported so a finding count is never mistaken for the
	// size of the estate.
	Declared  int
	Installed int
}

// kindOrder sorts findings by how much they undermine reproducibility, worst first. Mutable
// sources top the list: they change under you with no version change to notice.
var kindOrder = map[Kind]int{
	KindMutableSource:   0,
	KindUnpinnable:      1,
	KindVersionMismatch: 2,
	KindUndeclared:      3,
	KindMissing:         4,
	KindUnpinned:        5,
	KindFirstParty:      6,
}

// Compare produces a drift report.
func Compare(inv content.Inventory, req requirements.File) Report {
	rep := Report{
		Declared:  len(req.Collections) + len(req.Roles),
		Installed: len(inv.Components),
	}

	declared := map[string]requirements.Declaration{}
	for _, d := range append(append([]requirements.Declaration{}, req.Collections...), req.Roles...) {
		declared[d.FQN()] = d
	}

	installed := map[string]content.Component{}
	for _, c := range inv.Components {
		installed[c.FQN()] = c
	}

	// Findings about declarations.
	for _, d := range append(append([]requirements.Declaration{}, req.Collections...), req.Roles...) {
		fqn := d.FQN()
		ck := content.KindCollection
		if d.Kind == requirements.KindRole {
			ck = content.KindRole
		}

		if d.Mutable() {
			detail := "declared from a source-control or URL source with no version or ref: " +
				"a reinstall takes whatever the branch head is that day"
			if d.IsDerived() {
				detail += "; the installed name was derived from the URL by convention"
			}
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindMutableSource, Component: fqn, KindOf: ck, Declared: d.Name, Detail: detail,
			})
		} else if !d.Pinned() {
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindUnpinned, Component: fqn, KindOf: ck, Declared: versionOrAny(d.Version),
				Detail: "no exact version declared: a reinstall may produce a different version",
			})
		}

		inst, ok := installed[fqn]
		if !ok {
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindMissing, Component: fqn, KindOf: ck, Declared: versionOrAny(d.Version),
				Detail: "declared but not present in the scanned tree",
			})
			continue
		}
		if d.Pinned() && inst.Version != "" && d.Version != inst.Version {
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindVersionMismatch, Component: fqn, KindOf: ck,
				Declared: d.Version, Installed: inst.Version,
				Detail: "the installed version is not the one declared",
			})
		}
	}

	// Findings about installed content.
	for _, c := range inv.Components {
		if firstParty(c) {
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindFirstParty, Component: c.FQN(), KindOf: c.Kind,
				Detail: "authored here rather than installed: versioned by this repository, not by Galaxy",
			})
			continue
		}
		if _, ok := declared[c.FQN()]; !ok {
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindUndeclared, Component: c.FQN(), KindOf: c.Kind, Installed: c.Version,
				Detail: "installed but declared nowhere: pulled in transitively or by hand, at a version nobody chose",
			})
		}
		if !c.Versioned() {
			rep.Findings = append(rep.Findings, Finding{
				Kind: KindUnpinnable, Component: c.FQN(), KindOf: c.Kind,
				Detail: "no version recorded on disk: it cannot be pinned, so it cannot be reproduced",
			})
		}
	}

	sort.SliceStable(rep.Findings, func(i, j int) bool {
		a, b := rep.Findings[i], rep.Findings[j]
		if kindOrder[a.Kind] != kindOrder[b.Kind] {
			return kindOrder[a.Kind] < kindOrder[b.Kind]
		}
		return a.Component < b.Component
	})

	return rep
}

// Counts returns the number of findings per kind.
func (r Report) Counts() map[Kind]int {
	counts := map[Kind]int{}
	for _, f := range r.Findings {
		counts[f.Kind]++
	}
	return counts
}

// Reproducible reports whether every installed component could be pinned and rebuilt — which is
// a question about the lockfile, not about requirements.yml.
//
// Deliberately NOT covered: undeclared and missing components. An undeclared component still has
// a version, so `lock` can pin it and a rebuild from the lockfile reproduces it; it is a
// governance problem, not a reproducibility one. A declared-but-absent component says nothing
// about reproducing what is actually here.
//
// What does break it: a mutable source (no version to pin), a component with no recorded version
// at all, and a version mismatch (the declaration and reality disagree, so neither can be
// trusted as the thing to rebuild). First-party content is excluded — it travels with the
// repository.
func (r Report) Reproducible() bool {
	for _, f := range r.Findings {
		switch f.Kind {
		case KindMutableSource, KindUnpinnable, KindVersionMismatch:
			return false
		}
	}
	return true
}

func versionOrAny(v string) string {
	if v == "" {
		return "(any)"
	}
	return v
}
