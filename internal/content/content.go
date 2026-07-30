// Package content models Ansible content installed on a control node — collections and
// legacy roles — as it exists on disk.
//
// The two kinds are deliberately not interchangeable. Collections carry per-file checksums;
// roles carry none. See ADR-0005.
package content

// Kind distinguishes the two forms of installable Ansible content.
type Kind string

const (
	KindCollection Kind = "collection"
	KindRole       Kind = "role"
)

// Tier records how much can be asserted about a component. It is not a quality judgement about
// the content — it describes the metadata that exists for it. See ADR-0005.
type Tier string

const (
	// TierChecksummed: a manifest records a sha256 for every file. Verifiable.
	TierChecksummed Tier = "checksummed"
	// TierNameVersionOnly: no integrity data exists anywhere for this kind of content.
	// Absence of checksums here is a property of the ecosystem, not of this tool.
	TierNameVersionOnly Tier = "name-version-only"
)

// Coverage records whether any vulnerability database indexes this component's ecosystem.
// An empty result from a database that does not cover an ecosystem is not evidence of
// cleanliness. See ADR-0006.
type Coverage string

const (
	CoverageCovered    Coverage = "covered"
	CoverageNotCovered Coverage = "not-covered"
	CoverageUnknown    Coverage = "unknown"
)

// Origin is where a component came from, so far as the installed tree records it.
type Origin string

const (
	OriginGalaxy  Origin = "galaxy" // installed from a Galaxy-compatible server
	OriginGit     Origin = "git"    // built from a git checkout; no commit is recorded on disk
	OriginLocal   Origin = "local"  // present in the tree without install metadata
	OriginUnknown Origin = "unknown"
)

// File is one entry from a collection's FILES.json. Directory entries are not represented.
type File struct {
	Path   string
	SHA256 string
}

// Component is a single installed collection or role.
//
// Only Namespace, Name and Version are treated as identity. Every other field sourced from a
// manifest is informational: real collections ship unedited skeleton placeholders in fields such
// as `repository`. See ADR-0002 §5.
type Component struct {
	Kind      Kind
	Namespace string
	Name      string
	// Version is empty when none is recorded — true for roles present in a tree without
	// install metadata. It is never defaulted to a placeholder.
	Version string

	Path     string
	Origin   Origin
	Tier     Tier
	Coverage Coverage

	// Dependencies maps a dependency's fully-qualified name to its declared version
	// constraint, exactly as written. Constraints are recorded, never resolved (ADR-0003).
	Dependencies map[string]string

	// Licenses as declared. Informational.
	Licenses []string

	// Files is populated only for TierChecksummed components.
	Files []File

	// ManifestFormat is the `format` value of the manifest this was parsed from, or 0 when the
	// content kind has no manifest.
	ManifestFormat int
}

// FQN is the fully-qualified name: "namespace.name".
func (c Component) FQN() string {
	if c.Namespace == "" {
		return c.Name
	}
	return c.Namespace + "." + c.Name
}

// Versioned reports whether a version was recorded on disk.
func (c Component) Versioned() bool { return c.Version != "" }

// Inventory is the result of scanning a tree.
type Inventory struct {
	Components []Component
	// Problems holds paths that looked like content but could not be parsed, with the reason.
	// Declining to parse is always preferred to guessing (ADR-0007).
	Problems []Problem
}

// Problem records content that was found but not inventoried.
type Problem struct {
	Path   string
	Reason string
}

// Counts summarises an inventory by kind, for output that must not let a component count be
// mistaken for a verified count (ADR-0005, ADR-0006).
func (inv Inventory) Counts() (collections, roles, checksummed, unversioned int) {
	for _, c := range inv.Components {
		switch c.Kind {
		case KindCollection:
			collections++
		case KindRole:
			roles++
		}
		if c.Tier == TierChecksummed {
			checksummed++
		}
		if !c.Versioned() {
			unversioned++
		}
	}
	return
}
