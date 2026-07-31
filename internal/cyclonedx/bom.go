// Package cyclonedx renders an inventory as a CycloneDX bill of materials.
//
// The document is deliberately explicit about its own limits. Ansible content is indexed by no
// vulnerability database, and roles carry no integrity data at all — a BOM that let either fact
// pass unstated would read as assurance it cannot provide. See ADR-0005 and ADR-0006.
package cyclonedx

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jrjsmrtn/ansible-bom/content"
	"github.com/jrjsmrtn/ansible-bom/internal/lockfile"
	"github.com/jrjsmrtn/ansible-bom/internal/purl"
)

// SpecVersion is the CycloneDX specification version emitted.
const SpecVersion = "1.6"

// propertyPrefix namespaces properties this tool defines. CycloneDX has no first-class field for
// assurance tier or vulnerability-database coverage, so both ride on properties.
const propertyPrefix = "ansible-bom:"

// BOM is a CycloneDX document.
type BOM struct {
	BOMFormat    string        `json:"bomFormat"`
	SpecVersion  string        `json:"specVersion"`
	SerialNumber string        `json:"serialNumber"`
	Version      int           `json:"version"`
	Metadata     Metadata      `json:"metadata"`
	Components   []Component   `json:"components"`
	Dependencies []Dependency  `json:"dependencies,omitempty"`
	Compositions []Composition `json:"compositions,omitempty"`
}

type Metadata struct {
	Timestamp  string     `json:"timestamp"`
	Tools      Tools      `json:"tools"`
	Properties []Property `json:"properties,omitempty"`
}

type Tools struct {
	Components []Component `json:"components"`
}

type Component struct {
	Type       string         `json:"type"`
	BOMRef     string         `json:"bom-ref,omitempty"`
	Name       string         `json:"name"`
	Version    string         `json:"version,omitempty"`
	PURL       string         `json:"purl,omitempty"`
	Licenses   []LicenseEntry `json:"licenses,omitempty"`
	Properties []Property     `json:"properties,omitempty"`
}

type LicenseEntry struct {
	License License `json:"license"`
}

// License carries either an SPDX id or a free-text name, never both — CycloneDX treats them as
// alternatives, and declaring a name as an id would assert an SPDX match we have not made.
type License struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Dependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

// Composition declares how complete a portion of the BOM is. Without it a partial inventory is
// indistinguishable from an exhaustive one, which is a false claim of completeness.
type Composition struct {
	Aggregate  string   `json:"aggregate"`
	Assemblies []string `json:"assemblies,omitempty"`
}

// Options configures BOM generation.
type Options struct {
	ToolName    string
	ToolVersion string
	Roots       []string
	// Now is injected so output is reproducible under test.
	Now func() time.Time
}

// New builds a CycloneDX BOM from an inventory.
func New(inv content.Inventory, opts Options) (BOM, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	serial, err := serialNumber()
	if err != nil {
		return BOM{}, err
	}

	b := BOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  SpecVersion,
		SerialNumber: serial,
		Version:      1,
		Metadata: Metadata{
			Timestamp: now().UTC().Format(time.RFC3339),
			Tools: Tools{Components: []Component{{
				Type: "application", Name: opts.ToolName, Version: opts.ToolVersion,
			}}},
			Properties: metadataProperties(inv, opts),
		},
	}

	refs := map[string]string{} // component FQN -> bom-ref
	for _, c := range inv.Components {
		ref := purl.For(c)
		refs[c.FQN()] = ref
		b.Components = append(b.Components, componentFor(c, ref))
	}

	b.Dependencies = dependencies(inv, refs)
	b.Compositions = compositions(inv, refs)

	return b, nil
}

func componentFor(c content.Component, ref string) Component {
	comp := Component{
		Type:    "library",
		BOMRef:  ref,
		Name:    c.FQN(),
		Version: c.Version,
		PURL:    ref,
		Properties: []Property{
			{Name: propertyPrefix + "kind", Value: string(c.Kind)},
			{Name: propertyPrefix + "assurance-tier", Value: string(c.Tier)},
			{Name: propertyPrefix + "vulnerability-coverage", Value: string(c.Coverage)},
			{Name: propertyPrefix + "origin", Value: string(c.Origin)},
		},
	}

	// State why the assurance is what it is, so absence of hashes cannot be read as this tool
	// having failed to collect them (ADR-0005).
	switch c.Tier {
	case content.TierChecksummed:
		if d := lockfile.ContentDigest(c); d != "" {
			// Deliberately a property, not `hashes`: this digest is computed over the file
			// manifest, not over a distributed artefact. Putting it in `hashes` would assert
			// something about an artefact we never saw.
			comp.Properties = append(comp.Properties,
				Property{Name: propertyPrefix + "content-digest", Value: d},
				Property{Name: propertyPrefix + "file-count", Value: fmt.Sprint(len(c.Files))})
		}
	case content.TierNameVersionOnly:
		comp.Properties = append(comp.Properties, Property{
			Name:  propertyPrefix + "assurance-note",
			Value: "no checksums exist for Ansible roles; this is a property of the ecosystem, not of the tool",
		})
	}

	if !c.Versioned() {
		comp.Properties = append(comp.Properties, Property{
			Name:  propertyPrefix + "version-note",
			Value: "no version recorded on disk; not defaulted to a placeholder",
		})
	}

	comp.Licenses = licenses(c.Licenses)

	sort.Slice(comp.Properties, func(i, j int) bool { return comp.Properties[i].Name < comp.Properties[j].Name })
	return comp
}

// spdxLike reports whether a declared licence looks like an SPDX identifier. Ansible manifests
// carry a list that is *supposed* to hold SPDX ids but frequently holds prose.
func spdxLike(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t") {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '.', r == '+':
		default:
			return false
		}
	}
	return true
}

func licenses(declared []string) []LicenseEntry {
	var out []LicenseEntry
	for _, l := range declared {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if spdxLike(l) {
			out = append(out, LicenseEntry{License: License{ID: l}})
		} else {
			out = append(out, LicenseEntry{License: License{Name: l}})
		}
	}
	return out
}

// dependencies builds the graph. Only components present in this BOM are linked: a dangling ref
// would claim knowledge of something never inventoried. Declared dependencies that are absent are
// visible through `drift`, which is where a missing dependency belongs.
func dependencies(inv content.Inventory, refs map[string]string) []Dependency {
	var deps []Dependency
	for _, c := range inv.Components {
		ref := refs[c.FQN()]
		var on []string
		for name := range c.Dependencies {
			if r, ok := refs[name]; ok {
				on = append(on, r)
			}
		}
		sort.Strings(on)
		deps = append(deps, Dependency{Ref: ref, DependsOn: on})
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Ref < deps[j].Ref })
	return deps
}

// compositions states completeness. Content that could not be parsed makes the inventory
// incomplete, and saying so is the difference between a partial BOM and a false claim.
func compositions(inv content.Inventory, refs map[string]string) []Composition {
	aggregate := "complete"
	if len(inv.Problems) > 0 {
		aggregate = "incomplete"
	}
	assemblies := make([]string, 0, len(refs))
	for _, r := range refs {
		assemblies = append(assemblies, r)
	}
	sort.Strings(assemblies)
	return []Composition{{Aggregate: aggregate, Assemblies: assemblies}}
}

func metadataProperties(inv content.Inventory, opts Options) []Property {
	collections, roles, checksummed, unversioned := inv.Counts()
	props := []Property{
		{Name: propertyPrefix + "purl-status", Value: purl.Status},
		{Name: propertyPrefix + "purl-proposal", Value: purl.ProposalURL},
		{
			Name: propertyPrefix + "vulnerability-coverage",
			Value: "none: no vulnerability database indexes Ansible collections or roles. " +
				"An empty result from a scanner is not evidence that these components are unaffected.",
		},
		{Name: propertyPrefix + "collections", Value: fmt.Sprint(collections)},
		{Name: propertyPrefix + "roles", Value: fmt.Sprint(roles)},
		{Name: propertyPrefix + "checksummed", Value: fmt.Sprint(checksummed)},
		{Name: propertyPrefix + "unversioned", Value: fmt.Sprint(unversioned)},
		{Name: propertyPrefix + "unparsed", Value: fmt.Sprint(len(inv.Problems))},
	}
	for _, r := range opts.Roots {
		props = append(props, Property{Name: propertyPrefix + "root", Value: r})
	}
	for _, p := range inv.Problems {
		props = append(props, Property{
			Name:  propertyPrefix + "unparsed-path",
			Value: p.Path + ": " + p.Reason,
		})
	}
	return props
}

// serialNumber generates a RFC 4122 version 4 URN, as CycloneDX requires.
func serialNumber() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating serial number: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Marshal renders the BOM as indented JSON.
func Marshal(b BOM) ([]byte, error) {
	out, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding BOM: %w", err)
	}
	return append(out, '\n'), nil
}
