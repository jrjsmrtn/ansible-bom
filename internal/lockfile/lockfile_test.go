package lockfile

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jrjsmrtn/ansible-bom/internal/content"
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
	for _, want := range []string{
		"No vulnerability database indexes",
		"Roles carry no checksums",
		"unpinnable",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("header does not mention %q", want)
		}
	}
}

func TestRequirementsProjection(t *testing.T) {
	l := New(sampleInventory(), "t", nil)
	out, omitted, err := Requirements(l)
	if err != nil {
		t.Fatalf("Requirements: %v", err)
	}

	if omitted != 1 {
		t.Errorf("omitted = %d, want 1", omitted)
	}

	var r struct {
		Collections []struct{ Name, Version string } `yaml:"collections"`
		Roles       []struct{ Name, Version string } `yaml:"roles"`
	}
	if err := yaml.Unmarshal(out, &r); err != nil {
		t.Fatalf("projection is not valid YAML: %v", err)
	}
	if len(r.Collections) != 2 || len(r.Roles) != 1 {
		t.Fatalf("projection = %d collections, %d roles; want 2, 1", len(r.Collections), len(r.Roles))
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
