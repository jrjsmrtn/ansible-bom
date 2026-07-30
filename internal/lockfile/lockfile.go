// Package lockfile renders an inventory as a lockfile: the record of exactly what is installed,
// so a control node can be rebuilt as it stands rather than as its requirements.yml wishes.
//
// Ansible Galaxy has no lockfile. Without one, a byte-identical playbook executes through
// different module code between runs, which voids the idempotency guarantee at the one layer
// nobody inspects. This package exists to close that.
package lockfile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jrjsmrtn/ansible-bom/internal/content"
)

// FormatVersion is the lockfile schema version. Consumers must refuse a version they do not
// understand rather than guess, for the same reason this tool gates on MANIFEST.json's format.
const FormatVersion = 1

// Lock is the serialised form of an inventory.
type Lock struct {
	Version     int      `yaml:"version"`
	GeneratedBy string   `yaml:"generated_by"`
	Roots       []string `yaml:"roots"`

	Collections []Entry `yaml:"collections"`
	Roles       []Entry `yaml:"roles"`

	// Unpinnable lists content that exists but carries no version, so it cannot be locked.
	// It is recorded rather than omitted: a lockfile that silently drops components would
	// overstate the reproducibility it provides.
	Unpinnable []Unpinnable `yaml:"unpinnable,omitempty"`

	Summary Summary `yaml:"summary"`
}

// Entry is one pinned component.
type Entry struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Origin  string `yaml:"origin"`
	Tier    string `yaml:"tier"`

	// Digest is a content digest over the component's file manifest, present only for
	// checksummed components. It detects a version being republished with different content —
	// which pinning a version alone does not.
	Digest string `yaml:"digest,omitempty"`

	// Dependencies are the constraints the component declares, recorded verbatim and never
	// resolved. They describe what the component asked for, not what it got.
	Dependencies map[string]string `yaml:"dependencies,omitempty"`
}

// Unpinnable is content present on disk with no version to pin.
type Unpinnable struct {
	Name   string `yaml:"name"`
	Kind   string `yaml:"kind"`
	Path   string `yaml:"path"`
	Reason string `yaml:"reason"`
}

// Summary states what the lockfile does and does not cover. Counts are separated so that a
// component total can never be mistaken for a verified total.
type Summary struct {
	Collections int `yaml:"collections"`
	Roles       int `yaml:"roles"`
	Pinned      int `yaml:"pinned"`
	Unpinnable  int `yaml:"unpinnable"`
	Checksummed int `yaml:"checksummed"`
	Problems    int `yaml:"problems"`
}

// unpinnableReason explains why a component carries no version.
const unpinnableReason = "no version recorded on disk: not installed from Galaxy, and its metadata declares none"

// New builds a Lock from an inventory.
func New(inv content.Inventory, generatedBy string, roots []string) Lock {
	l := Lock{
		Version:     FormatVersion,
		GeneratedBy: generatedBy,
		Roots:       roots,
	}

	for _, c := range inv.Components {
		if !c.Versioned() {
			l.Unpinnable = append(l.Unpinnable, Unpinnable{
				Name:   c.FQN(),
				Kind:   string(c.Kind),
				Path:   c.Path,
				Reason: unpinnableReason,
			})
			continue
		}

		e := Entry{
			Name:         c.FQN(),
			Version:      c.Version,
			Origin:       string(c.Origin),
			Tier:         string(c.Tier),
			Dependencies: c.Dependencies,
		}
		if len(e.Dependencies) == 0 {
			e.Dependencies = nil
		}
		if c.Tier == content.TierChecksummed {
			e.Digest = ContentDigest(c)
		}

		switch c.Kind {
		case content.KindCollection:
			l.Collections = append(l.Collections, e)
		case content.KindRole:
			l.Roles = append(l.Roles, e)
		}
	}

	sort.Slice(l.Collections, func(i, j int) bool { return l.Collections[i].Name < l.Collections[j].Name })
	sort.Slice(l.Roles, func(i, j int) bool { return l.Roles[i].Name < l.Roles[j].Name })
	sort.Slice(l.Unpinnable, func(i, j int) bool { return l.Unpinnable[i].Name < l.Unpinnable[j].Name })

	collections, roles, checksummed, _ := inv.Counts()
	l.Summary = Summary{
		Collections: collections,
		Roles:       roles,
		Pinned:      len(l.Collections) + len(l.Roles),
		Unpinnable:  len(l.Unpinnable),
		Checksummed: checksummed,
		Problems:    len(inv.Problems),
	}

	return l
}

// ContentDigest is a stable digest over a component's file manifest: sha256 of the sorted
// "path\x00sha256\n" lines from FILES.json.
//
// It is derived from the checksums ansible-galaxy recorded at build time, so it inherits their
// trust — it attests that the manifest is unchanged, not that the files match the manifest.
// Checking files against the manifest is what `verify` does.
func ContentDigest(c content.Component) string {
	if len(c.Files) == 0 {
		return ""
	}
	files := make([]content.File, len(c.Files))
	copy(files, c.Files)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%s\n", f.Path, f.SHA256)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// header is prepended to the rendered lockfile. It states the limits of what the file asserts,
// where a reader will actually see them.
const header = `# ansible-bom lockfile — the content installed on this control node, as it stands.
#
# Regenerate with 'ansible-bom lock'. Do not hand-edit: it describes observed state, not intent.
# Your requirements.yml remains the file you edit.
#
# What this file asserts:
#   - the exact version of every component that had one recorded on disk
#   - a content digest for collections, derived from the checksums recorded at install time
#
# What it does NOT assert:
#   - that any component is free of known vulnerabilities. No vulnerability database indexes
#     Ansible collections or roles; an empty result from one is not evidence of anything.
#   - integrity for roles. Roles carry no checksums anywhere — this is a property of the
#     ecosystem, not a limitation of this tool.
#   - completeness, where an 'unpinnable' section is present: that content exists but cannot
#     be pinned, and a rebuild from this file will not reproduce it.
`

// Marshal renders the lockfile.
func Marshal(l Lock) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(header)

	enc, err := yaml.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("encoding lockfile: %w", err)
	}
	sb.WriteString("\n")
	sb.Write(enc)
	return []byte(sb.String()), nil
}
