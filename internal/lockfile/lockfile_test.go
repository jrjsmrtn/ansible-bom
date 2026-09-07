// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package lockfile

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jrjsmrtn/ansible-bom/content"
)

func sampleInventory() content.Inventory {
	return content.Inventory{
		Components: []content.Component{
			{
				Kind: content.KindCollection, Namespace: "community", Name: "general",
				Version: "11.4.0", Origin: content.OriginGalaxy, Tier: content.TierChecksummed,
				Files: []content.File{
					{Path: "plugins/a.py", SHA256: "aaa"},
					{Path: "plugins/b.py", SHA256: "bbb"},
				},
			},
			{
				Kind: content.KindCollection, Namespace: "community", Name: "windows",
				Version: "3.0.1", Origin: content.OriginUnknown, Tier: content.TierChecksummed,
				Dependencies: map[string]string{"ansible.windows": ">=3.0.0,<4.0.0"},
				Files:        []content.File{{Path: "plugins/c.py", SHA256: "ccc"}},
			},
			{
				Kind: content.KindRole, Namespace: "jborean93", Name: "win_openssh",
				Version: "0.3.2", Origin: content.OriginGalaxy, Tier: content.TierNameVersionOnly,
			},
			{
				// No version: cannot be pinned.
				Kind: content.KindRole, Name: "site_common", Path: "/roles/site_common",
				Origin: content.OriginLocal, Tier: content.TierNameVersionOnly,
			},
		},
		Problems: []content.Problem{{Path: "/x/broken", Reason: "unsupported format"}},
	}
}

func TestNew(t *testing.T) {
	l := New(sampleInventory(), "ansible-bom test", []string{"/tmp/root"})

	if l.Version != FormatVersion {
		t.Errorf("Version = %d, want %d", l.Version, FormatVersion)
	}
	if len(l.Collections) != 2 {
		t.Errorf("Collections = %d, want 2", len(l.Collections))
	}
	if len(l.Roles) != 1 {
		t.Errorf("Roles = %d, want 1 (the versioned one)", len(l.Roles))
	}
	if len(l.Unpinnable) != 1 || l.Unpinnable[0].Name != "site_common" {
		t.Fatalf("Unpinnable = %v, want exactly site_common", l.Unpinnable)
	}
	if l.Summary.Pinned != 3 || l.Summary.Unpinnable != 1 {
		t.Errorf("Summary pinned/unpinnable = %d/%d, want 3/1", l.Summary.Pinned, l.Summary.Unpinnable)
	}
	if l.Summary.Checksummed != 2 {
		t.Errorf("Summary.Checksummed = %d, want 2 — roles are never checksummed", l.Summary.Checksummed)
	}
	if l.Summary.Problems != 1 {
		t.Errorf("Summary.Problems = %d, want 1", l.Summary.Problems)
	}
}

// A component with no version must appear in the lockfile as unpinnable, never be dropped and
// never be given an invented version. Silently omitting it would overstate reproducibility.
func TestUnversionedContentIsRecordedNotDropped(t *testing.T) {
	l := New(sampleInventory(), "t", nil)

	for _, e := range append(l.Collections, l.Roles...) {
		if e.Name == "site_common" {
			t.Fatal("unversioned role was pinned")
		}
		if e.Version == "" {
			t.Errorf("%s pinned with an empty version", e.Name)
		}
	}
	if l.Unpinnable[0].Reason == "" {
		t.Error("unpinnable entry carries no reason")
	}
}

// Roles never get a digest: there is nothing to derive one from.
func TestOnlyChecksummedComponentsGetDigests(t *testing.T) {
	l := New(sampleInventory(), "t", nil)
	for _, e := range l.Collections {
		if e.Digest == "" {
			t.Errorf("collection %s has no digest", e.Name)
		}
		if !strings.HasPrefix(e.Digest, "sha256:") {
			t.Errorf("collection %s digest = %q, want a sha256: prefix", e.Name, e.Digest)
		}
	}
	for _, e := range l.Roles {
		if e.Digest != "" {
			t.Errorf("role %s has digest %q — roles carry no checksums", e.Name, e.Digest)
		}
	}
}

func TestContentDigestIsStableAndOrderIndependent(t *testing.T) {
	a := content.Component{Files: []content.File{
		{Path: "b", SHA256: "2"}, {Path: "a", SHA256: "1"},
	}}
	b := content.Component{Files: []content.File{
		{Path: "a", SHA256: "1"}, {Path: "b", SHA256: "2"},
	}}
	if ContentDigest(a) != ContentDigest(b) {
		t.Error("digest depends on file order")
	}

	// A changed checksum must change the digest — that is the point of it.
	c := content.Component{Files: []content.File{
		{Path: "a", SHA256: "1"}, {Path: "b", SHA256: "CHANGED"},
	}}
	if ContentDigest(b) == ContentDigest(c) {
		t.Error("digest unchanged despite a changed file checksum")
	}

	if ContentDigest(content.Component{}) != "" {
		t.Error("a component with no files should have no digest")
	}
}

func TestMarshalIsValidYAMLAndStatesItsLimits(t *testing.T) {
	l := New(sampleInventory(), "ansible-bom test", []string{"/tmp/root"})
	out, err := Marshal(l)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var round Lock
	if err := yaml.Unmarshal(out, &round); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
	if round.Version != FormatVersion || len(round.Collections) != 2 {
		t.Errorf("round trip lost data: %+v", round.Summary)
	}

	// The header must say what the file does not assert, where a reader will see it.
	//
	// Matched against the header with comment markers and line breaks flattened away: the claim
	// is that the text says these things, not that it wraps at any particular column. Pinning the
	// line breaks made this fail when the header was rewrapped to satisfy yamllint (issue #8),
	// which is a false alarm about formatting dressed up as a claim about content.
	flat := flattenComments(string(out))
	for _, want := range []string{
		"No vulnerability database indexes",
		"Roles carry no checksums",
		"unpinnable",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("header does not mention %q", want)
		}
	}
}

// flattenComments reduces a comment block to one whitespace-normalised line, so an assertion
// about what the text SAYS is not also an assertion about where it wraps.
func flattenComments(doc string) string {
	var b strings.Builder
	for _, l := range strings.Split(doc, "\n") {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		b.WriteString(strings.TrimSpace(strings.TrimPrefix(t, "#")))
		b.WriteString(" ")
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func TestRequirementsProjection(t *testing.T) {
	l := New(sampleInventory(), "t", nil)
	out, omitted, err := Requirements(l, nil)
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}

	if len(omitted.Unpinnable) != 1 {
		t.Errorf("Unpinnable = %d, want 1", len(omitted.Unpinnable))
	}
	if len(omitted.OffGalaxy) != 1 {
		t.Errorf("OffGalaxy = %d, want 1", len(omitted.OffGalaxy))
	}
	if omitted.Len() != 2 {
		t.Errorf("Len = %d, want 2", omitted.Len())
	}

	var r struct {
		Collections []struct{ Name, Version string } `yaml:"collections"`
		Roles       []struct{ Name, Version string } `yaml:"roles"`
	}
	if err := yaml.Unmarshal(out, &r); err != nil {
		t.Fatalf("projection is not valid YAML: %v", err)
	}
	if len(r.Collections) != 1 || len(r.Roles) != 1 {
		t.Fatalf("projection = %d collections, %d roles; want 1, 1", len(r.Collections), len(r.Roles))
	}
	for _, c := range append(r.Collections, r.Roles...) {
		if c.Version == "" {
			t.Errorf("%s projected without a version", c.Name)
		}
	}

	// The omission must be visible in the file itself, not only in the return value — someone
	// will read this file without ever seeing the tool's output.
	if !strings.Contains(string(out), "OMITTED") || !strings.Contains(string(out), "site_common") {
		t.Error("projection does not name the omitted component")
	}
}

// Issue #7: a component whose origin is not galaxy was projected as a bare Galaxy requirement.
// ansible-galaxy resolves the file as one dependency problem, so the unresolvable entry aborted
// the whole install and NOTHING was installed — including the components that were fine.
//
// community.windows in the fixture is exactly that shape: versioned, tier checksummed, with a
// digest, so it passes the unpinnable filter, and origin unknown, so Galaxy cannot resolve it.
func TestRequirementsOmitsComponentsGalaxyCannotResolve(t *testing.T) {
	l := New(sampleInventory(), "t", nil)
	out, omitted, err := Requirements(l, nil)
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}

	var r struct {
		Collections []struct{ Name, Version string } `yaml:"collections"`
		Roles       []struct{ Name, Version string } `yaml:"roles"`
	}
	if err := yaml.Unmarshal(out, &r); err != nil {
		t.Fatalf("projection is not valid YAML: %v", err)
	}

	// Nothing that Galaxy cannot resolve may appear as a requirement.
	byName := map[string]bool{}
	for _, c := range append(r.Collections, r.Roles...) {
		byName[c.Name] = true
	}
	for _, e := range append(append([]Entry{}, l.Collections...), l.Roles...) {
		emitted := byName[e.Name]
		installable := e.Origin == galaxyOrigin
		if emitted != installable {
			t.Errorf("%s: origin %q, emitted as a requirement = %v, want %v",
				e.Name, e.Origin, emitted, installable)
		}
	}

	// ...and it must be reported rather than dropped, in both channels.
	if len(omitted.OffGalaxy) != 1 || omitted.OffGalaxy[0].Name != "community.windows" {
		t.Fatalf("OffGalaxy = %+v, want community.windows", omitted.OffGalaxy)
	}
	if !strings.Contains(string(out), "community.windows 3.0.1 (origin: unknown)") {
		t.Error("the file does not name the off-Galaxy component and its origin")
	}
	if !strings.Contains(string(out), "not installable from Galaxy by name") {
		t.Error("the header does not explain why it was left out")
	}
}

// The filter must key on origin alone. A component can be checksummed, versioned and carry a
// digest — every signal of being well-known — and still be unresolvable by name.
func TestRequirementsFilterKeysOnOriginNotAssurance(t *testing.T) {
	for _, tc := range []struct {
		origin  string
		emitted bool
	}{
		{"galaxy", true},
		{"git", false},
		{"local", false},
		{"unknown", false},
	} {
		l := Lock{Collections: []Entry{{
			Name: "ns.thing", Version: "1.0.0", Origin: tc.origin,
			Tier: "checksummed", Digest: "sha256:abc",
		}}}
		out, omitted, err := Requirements(l, nil)
		if err != nil {
			t.Fatalf("origin %s: Requirements: %v", tc.origin, err)
		}
		var r struct {
			Collections []struct{ Name string } `yaml:"collections"`
		}
		if err := yaml.Unmarshal(out, &r); err != nil {
			t.Fatalf("origin %s: not valid YAML: %v", tc.origin, err)
		}
		if got := len(r.Collections) == 1; got != tc.emitted {
			t.Errorf("origin %q: emitted = %v, want %v", tc.origin, got, tc.emitted)
		}
		if got := len(omitted.OffGalaxy) == 0; got != tc.emitted {
			t.Errorf("origin %q: reported as installable = %v, want %v", tc.origin, got, tc.emitted)
		}
	}
}

// Issue #8: the emitted files must pass yamllint at its defaults, so a repository that lints its
// YAML on commit does not have to carve out an exemption for a file it must not hand-edit.
//
// These assert the three properties yamllint checked, in Go, so the suite catches a regression
// without depending on yamllint being installed. The linters themselves were run against real
// output when this landed; that is the check these encode, not replace.
const yamlLineLimit = 88

// lintFaults reports the yamllint violations these files must never carry. Line length is counted
// in RUNES, not bytes — the headers contain em dashes, and counting bytes overstates the length
// of exactly the lines most likely to be near the limit.
func lintFaults(doc string) []string {
	var faults []string
	lines := strings.Split(doc, "\n")

	sawDocStart := false
	for i, l := range lines {
		if n := len([]rune(l)); n > yamlLineLimit {
			faults = append(faults, fmt.Sprintf("line %d: %d chars (limit %d)", i+1, n, yamlLineLimit))
		}
		if l == "---" {
			sawDocStart = true
			continue
		}
		// Content before the document start marker, comments aside, means it is misplaced.
		if !sawDocStart && l != "" && !strings.HasPrefix(strings.TrimSpace(l), "#") {
			faults = append(faults, fmt.Sprintf("line %d: content before the document start", i+1))
		}
	}
	if !sawDocStart {
		faults = append(faults, `missing document start "---"`)
	}
	return faults
}

func TestEmittedYAMLIsLintClean(t *testing.T) {
	l := New(sampleInventory(), "ansible-bom test", []string{"/srv/content"})

	lock, err := Marshal(l)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	req, _, err := Requirements(l, nil)
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}

	for name, doc := range map[string]string{"lockfile": string(lock), "requirements": string(req)} {
		for _, f := range lintFaults(doc) {
			t.Errorf("%s: %s", name, f)
		}
	}
}

// Two-space indentation is what puts a mapping nested under a sequence item at an indent relative
// to its parent key. yaml.v3 defaults to four, which yamllint reads as under-indented — the one
// fault in issue #8 that ansible-lint rated fatal rather than a warning.
func TestNestedMappingIndentation(t *testing.T) {
	out, err := Marshal(New(sampleInventory(), "t", nil))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	const want = "  - name: community.windows\n"
	if !strings.Contains(string(out), want) {
		t.Errorf("sequence item not indented by %d: want %q", yamlIndent, want)
	}
	if !strings.Contains(string(out), "    dependencies:\n      ansible.windows:") {
		t.Error("nested mapping is not indented relative to its parent key")
	}
}

// Proves the checker can fail, on each fault it is meant to catch.
func TestLintFaultsRejectsBadDocuments(t *testing.T) {
	for name, doc := range map[string]string{
		"no document start": "version: 1\n",
		"content first":     "version: 1\n---\n",
		"long line":         "---\n# " + strings.Repeat("x", yamlLineLimit) + "\n",
	} {
		if len(lintFaults(doc)) == 0 {
			t.Errorf("%s: accepted a document it should reject", name)
		}
	}
	if f := lintFaults("# ok\n---\nversion: 1\n"); len(f) != 0 {
		t.Errorf("rejected a clean document: %v", f)
	}
}

// Issue #9. A component installed from git is omitted from the projection because the tree
// records no trustworthy source — but the requirements.yml the operator wrote does. Carrying that
// declaration through is the difference between an incomplete file and a complete one.
func TestRequirementsCarriesDeclaredSources(t *testing.T) {
	l := New(sampleInventory(), "t", nil)
	declared := map[string]Declared{
		"community.windows": {
			Name: "community.windows", Version: "main",
			Type: "git", Source: "https://github.com/example/community.windows.git",
		},
	}

	out, om, err := Requirements(l, declared)
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}

	var r struct {
		Collections []struct{ Name, Version, Type, Source string } `yaml:"collections"`
	}
	if err := yaml.Unmarshal(out, &r); err != nil {
		t.Fatalf("projection is not valid YAML: %v", err)
	}

	var got *struct{ Name, Version, Type, Source string }
	for i := range r.Collections {
		if r.Collections[i].Name == "community.windows" {
			got = &r.Collections[i]
		}
	}
	if got == nil {
		t.Fatalf("the declared component was not carried through: %+v", r.Collections)
	}
	// Verbatim: this tool did not derive any of it.
	if got.Type != "git" || got.Source != "https://github.com/example/community.windows.git" || got.Version != "main" {
		t.Errorf("carried entry = %+v, want the declaration verbatim", *got)
	}

	// It is in the file, and it is still not reproducible — both must be said.
	if len(om.NotImmutable) != 1 || om.NotImmutable[0].Name != "community.windows" {
		t.Errorf("NotImmutable = %+v, want community.windows", om.NotImmutable)
	}
	for _, want := range []string{"NOT immutably pinned", "community.windows"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the file does not say %q", want)
		}
	}
	// Carried through, so no longer omitted.
	for _, e := range om.OffGalaxy {
		if e.Name == "community.windows" {
			t.Error("a carried-through component is still listed as omitted")
		}
	}
}

// Only a full commit SHA is treated as a pin. A tag can be moved or deleted upstream, so calling
// one immutable would promise reproducibility this tool cannot check.
func TestDeclaredImmutability(t *testing.T) {
	for version, want := range map[string]bool{
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4": true,
		"E3B0C44298FC1C149AFBF4C8996FB92427AE41E4": true,
		"v1.2.3": false,
		"main":   false,
		"":       false,
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e":  false, // 39 chars
		"e3b0c44298fc1c149afbf4c8996fb92427ae41eZ": false,
	} {
		if got := (Declared{Version: version}).Immutable(); got != want {
			t.Errorf("Immutable(%q) = %v, want %v", version, got, want)
		}
	}
}
