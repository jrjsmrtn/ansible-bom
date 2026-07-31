package cyclonedx

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jrjsmrtn/ansible-bom/content"
)

func fixedTime() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC) }

func opts() Options {
	return Options{ToolName: "ansible-bom", ToolVersion: "test", Roots: []string{"/srv/content"}, Now: fixedTime}
}

func sample() content.Inventory {
	return content.Inventory{
		Components: []content.Component{
			{
				Kind: content.KindCollection, Namespace: "community", Name: "windows",
				Version: "3.0.1", Origin: content.OriginGalaxy, Tier: content.TierChecksummed,
				Coverage:     content.CoverageNotCovered,
				Licenses:     []string{"GPL-3.0-or-later"},
				Dependencies: map[string]string{"ansible.windows": ">=3.0.0,<4.0.0", "absent.thing": "*"},
				Files:        []content.File{{Path: "a", SHA256: "1"}},
			},
			{
				Kind: content.KindCollection, Namespace: "ansible", Name: "windows",
				Version: "3.2.0", Origin: content.OriginGalaxy, Tier: content.TierChecksummed,
				Coverage: content.CoverageNotCovered,
				Licenses: []string{"See LICENSE file"},
				Files:    []content.File{{Path: "b", SHA256: "2"}},
			},
			{
				Kind: content.KindRole, Name: "site_common",
				Origin: content.OriginLocal, Tier: content.TierNameVersionOnly,
				Coverage: content.CoverageNotCovered,
			},
		},
	}
}

func build(t *testing.T, inv content.Inventory) BOM {
	t.Helper()
	b, err := New(inv, opts())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestBOMEnvelope(t *testing.T) {
	b := build(t, sample())
	if b.BOMFormat != "CycloneDX" || b.SpecVersion != SpecVersion {
		t.Errorf("envelope = %s/%s", b.BOMFormat, b.SpecVersion)
	}
	if !strings.HasPrefix(b.SerialNumber, "urn:uuid:") || len(b.SerialNumber) != 45 {
		t.Errorf("SerialNumber = %q, want a urn:uuid", b.SerialNumber)
	}
	if b.Metadata.Timestamp != "2026-07-31T12:00:00Z" {
		t.Errorf("Timestamp = %q", b.Metadata.Timestamp)
	}
	if len(b.Components) != 3 {
		t.Errorf("Components = %d, want 3", len(b.Components))
	}
}

// Every component must carry its assurance tier and coverage status. Without them a reader
// cannot distinguish "no hashes collected" from "no hashes exist" (ADR-0005, ADR-0006).
func TestEveryComponentDeclaresTierAndCoverage(t *testing.T) {
	b := build(t, sample())
	for _, c := range b.Components {
		props := map[string]string{}
		for _, p := range c.Properties {
			props[p.Name] = p.Value
		}
		if props[propertyPrefix+"assurance-tier"] == "" {
			t.Errorf("%s: no assurance-tier property", c.Name)
		}
		if props[propertyPrefix+"vulnerability-coverage"] != string(content.CoverageNotCovered) {
			t.Errorf("%s: vulnerability-coverage = %q, want not-covered",
				c.Name, props[propertyPrefix+"vulnerability-coverage"])
		}
	}
}

// A role must say that no checksums exist, rather than merely having none.
func TestRolesExplainTheirMissingIntegrityData(t *testing.T) {
	b := build(t, sample())
	for _, c := range b.Components {
		var kind, note, digest string
		for _, p := range c.Properties {
			switch p.Name {
			case propertyPrefix + "kind":
				kind = p.Value
			case propertyPrefix + "assurance-note":
				note = p.Value
			case propertyPrefix + "content-digest":
				digest = p.Value
			}
		}
		if kind != string(content.KindRole) {
			continue
		}
		if digest != "" {
			t.Errorf("%s: role carries a content digest", c.Name)
		}
		if !strings.Contains(note, "property of the ecosystem") {
			t.Errorf("%s: assurance-note = %q, want it to attribute the gap to the ecosystem", c.Name, note)
		}
	}
}

// The digest belongs in a property, not in `hashes`: it is computed over the file manifest, not
// over a distributed artefact, and `hashes` would assert something we never observed.
func TestDigestIsNotPresentedAsAnArtefactHash(t *testing.T) {
	out, err := Marshal(build(t, sample()))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(out), `"hashes"`) {
		t.Error("BOM contains a hashes field; the content digest must not be presented as an artefact hash")
	}
	if !strings.Contains(string(out), propertyPrefix+"content-digest") {
		t.Error("BOM carries no content-digest property")
	}
}

// Dangling dependency refs would claim knowledge of components never inventoried.
func TestDependenciesOnlyLinkPresentComponents(t *testing.T) {
	b := build(t, sample())
	present := map[string]bool{}
	for _, c := range b.Components {
		present[c.BOMRef] = true
	}
	var linked int
	for _, d := range b.Dependencies {
		if !present[d.Ref] {
			t.Errorf("dependency ref %q is not a component in this BOM", d.Ref)
		}
		for _, on := range d.DependsOn {
			if !present[on] {
				t.Errorf("%s dependsOn %q, which is not in this BOM", d.Ref, on)
			}
			linked++
		}
	}
	// community.windows declares two dependencies, only one of which is installed.
	if linked != 1 {
		t.Errorf("linked dependencies = %d, want 1 (absent.thing must not be linked)", linked)
	}
}

// A BOM that could not inventory everything must say so, or it claims a completeness it lacks.
func TestCompositionsDeclareCompleteness(t *testing.T) {
	b := build(t, sample())
	if got := b.Compositions[0].Aggregate; got != "complete" {
		t.Errorf("aggregate = %q, want complete when nothing failed to parse", got)
	}

	inv := sample()
	inv.Problems = []content.Problem{{Path: "/x", Reason: "unsupported format"}}
	b = build(t, inv)
	if got := b.Compositions[0].Aggregate; got != "incomplete" {
		t.Errorf("aggregate = %q, want incomplete when content could not be parsed", got)
	}
}

// Identifiers are provisional until the purl type is registered; the document must say so.
func TestMetadataDeclaresProvisionalIdentifiers(t *testing.T) {
	b := build(t, sample())
	var status, coverage string
	for _, p := range b.Metadata.Properties {
		switch p.Name {
		case propertyPrefix + "purl-status":
			status = p.Value
		case propertyPrefix + "vulnerability-coverage":
			coverage = p.Value
		}
	}
	if !strings.Contains(status, "provisional") {
		t.Errorf("purl-status = %q", status)
	}
	if !strings.Contains(coverage, "not evidence") {
		t.Errorf("metadata coverage note = %q, want it to warn that an empty result proves nothing", coverage)
	}
}

// A licence string that is prose must not be emitted as an SPDX identifier.
func TestLicencesDistinguishIdentifiersFromProse(t *testing.T) {
	b := build(t, sample())
	for _, c := range b.Components {
		for _, l := range c.Licenses {
			if l.License.ID != "" && l.License.Name != "" {
				t.Errorf("%s: licence carries both id and name", c.Name)
			}
			if strings.Contains(l.License.ID, " ") {
				t.Errorf("%s: prose emitted as an SPDX id: %q", c.Name, l.License.ID)
			}
		}
	}
}

func TestMarshalIsValidJSON(t *testing.T) {
	out, err := Marshal(build(t, sample()))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, k := range []string{"bomFormat", "specVersion", "serialNumber", "version", "metadata", "components"} {
		if _, ok := round[k]; !ok {
			t.Errorf("missing required field %q", k)
		}
	}
}

// A nil slice marshals to `null`, and the CycloneDX schema requires `components` to be an array.
// An empty inventory must still produce a valid document — CI caught this by scanning a path that
// no longer existed and getting "components: None is not of type 'array'".
func TestEmptyInventoryProducesAValidComponentsArray(t *testing.T) {
	b := build(t, content.Inventory{})
	if b.Components == nil {
		t.Fatal("Components is nil; it must be an empty array")
	}

	out, err := Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := raw["components"].([]any); !ok {
		t.Errorf("components serialised as %T, want an array", raw["components"])
	}
}
