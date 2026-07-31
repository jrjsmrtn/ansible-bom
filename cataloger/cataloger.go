// Package cataloger implements syft catalogers for Ansible content.
//
// It is a SEPARATE Go module, deliberately. Importing syft takes a dependency graph from 3
// modules to 445; the ansible-bom CLI emits CycloneDX natively and gains nothing from it, so the
// shipped binary must not carry that weight. See ADR-0008.
//
// The parsing logic is not duplicated here: it comes from
// github.com/jrjsmrtn/ansible-bom/content, whose decoders are reader-based precisely so they can
// serve both a filesystem walk and a syft parser.
//
// Two catalogers, following syft's declared/installed convention:
//
//	ansible-collection-cataloger   installed   MANIFEST.json in an ansible_collections tree
//	ansible-role-cataloger         installed   meta/main.yml in a roles tree
//
// A third — ansible-requirements-cataloger, declared, over requirements.yml — is deliberately not
// implemented yet. Whether syft wants a `declared` cataloger for a file that names version ranges
// rather than versions is one of the questions put to maintainers in anchore/syft#5129.
package cataloger

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/pkg/cataloger/generic"

	"github.com/jrjsmrtn/ansible-bom/content"
)

const (
	// CollectionCatalogerName follows syft's <ecosystem>-<source>-cataloger convention.
	CollectionCatalogerName = "ansible-collection-cataloger"
	// RoleCatalogerName is its legacy-role counterpart.
	RoleCatalogerName = "ansible-role-cataloger"

	// Type is the proposed purl type. There is no registered `ansible` type yet —
	// package-url/purl-spec#854 proposes one and is still open — so identifiers emitted here
	// are provisional, which is the first question put to syft maintainers.
	Type = "ansible"
)

// NewCollectionCataloger returns a cataloger for installed Ansible collections.
func NewCollectionCataloger() pkg.Cataloger {
	return generic.NewCataloger(CollectionCatalogerName).
		WithParserByGlobs(parseCollection, "**/ansible_collections/*/*/MANIFEST.json")
}

// NewRoleCataloger returns a cataloger for installed legacy Ansible roles.
func NewRoleCataloger() pkg.Cataloger {
	return generic.NewCataloger(RoleCatalogerName).
		WithParserByGlobs(parseRole, "**/roles/*/meta/main.yml", "**/roles/*/meta/main.yaml")
}

// parseCollection is invoked for each MANIFEST.json the glob matches.
//
// FILES.json is fetched through the resolver rather than assumed: a collection whose manifest is
// present without it is not a shape ansible-galaxy produces, and reporting nothing is better than
// emitting a package that appears to have no integrity data when the real answer is "this tree is
// malformed".
func parseCollection(
	_ context.Context, resolver file.Resolver, _ *generic.Environment, reader file.LocationReadCloser,
) ([]pkg.Package, []artifact.Relationship, error) {
	manifest, err := content.DecodeManifest(reader)
	if err != nil {
		// Declining to parse is deliberate: MANIFEST.json has no schema, so a shape we do not
		// recognise must not be guessed at (ADR-0007).
		return nil, nil, fmt.Errorf("%s: %w", reader.Location.RealPath, err)
	}

	c := content.ComponentFromManifest(manifest)
	c.Path = path.Dir(reader.Location.RealPath)

	if files, err := readFiles(resolver, c.Path); err == nil {
		c.Files = files
	}

	p := newPackage(c, reader.Location)
	return []pkg.Package{p}, nil, nil
}

// parseRole is invoked for each meta/main.yml the glob matches.
func parseRole(
	_ context.Context, resolver file.Resolver, _ *generic.Environment, reader file.LocationReadCloser,
) ([]pkg.Package, []artifact.Relationship, error) {
	meta, err := content.DecodeRoleMeta(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", reader.Location.RealPath, err)
	}

	// <roles>/<name>/meta/main.yml — the role directory is two levels up, and its name is the
	// only reliable source of identity.
	metaDir := path.Dir(reader.Location.RealPath)
	roleDir := path.Dir(metaDir)

	installed := readInstallInfo(resolver, metaDir)

	c := content.ComponentFromRoleMeta(path.Base(roleDir), meta, installed)
	c.Path = roleDir

	return []pkg.Package{newPackage(c, reader.Location)}, nil, nil
}

func readFiles(resolver file.Resolver, dir string) ([]content.File, error) {
	locs, err := resolver.FilesByPath(path.Join(dir, "FILES.json"))
	if err != nil || len(locs) == 0 {
		return nil, fmt.Errorf("FILES.json not resolvable under %s", dir)
	}
	rc, err := resolver.FileContentsByLocation(locs[0])
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return content.DecodeFiles(rc)
}

func readInstallInfo(resolver file.Resolver, metaDir string) string {
	locs, err := resolver.FilesByPath(path.Join(metaDir, ".galaxy_install_info"))
	if err != nil || len(locs) == 0 {
		return ""
	}
	rc, err := resolver.FileContentsByLocation(locs[0])
	if err != nil {
		return ""
	}
	defer rc.Close()
	v, _ := content.DecodeInstallInfo(rc)
	return v
}

// newPackage converts a component into a syft package.
//
// Metadata carries the two facts a consumer cannot recover from the package alone: that roles have
// no integrity data anywhere, and that no vulnerability database indexes Ansible content. Both are
// properties of the ecosystem rather than limitations of the scan, and a BOM that lets either pass
// unstated reads as assurance it cannot provide (ADR-0005, ADR-0006).
func newPackage(c content.Component, loc file.Location) pkg.Package {
	p := pkg.Package{
		Name:      c.FQN(),
		Version:   c.Version,
		Locations: file.NewLocationSet(loc.WithAnnotation(pkg.EvidenceAnnotationKey, pkg.PrimaryEvidenceAnnotation)),
		Type:      pkg.UnknownPkg,
		Licenses:  pkg.NewLicenseSet(licenses(c, loc)...),
		PURL:      purl(c),
		Metadata:  newMetadata(c),
	}
	p.SetID()
	return p
}

// Metadata is the syft metadata payload for an Ansible component.
type Metadata struct {
	Kind          string            `json:"kind"`
	AssuranceTier string            `json:"assuranceTier"`
	AssuranceNote string            `json:"assuranceNote,omitempty"`
	Coverage      string            `json:"vulnerabilityCoverage"`
	CoverageNote  string            `json:"vulnerabilityCoverageNote"`
	Origin        string            `json:"origin"`
	Dependencies  map[string]string `json:"dependencies,omitempty"`
	FileCount     int               `json:"fileCount,omitempty"`
}

const coverageNote = "no vulnerability database indexes Ansible collections or roles; " +
	"an empty scanner result is not evidence that this component is unaffected"

const roleAssuranceNote = "no checksums exist for Ansible roles; " +
	"this is a property of the ecosystem, not of the scan"

func newMetadata(c content.Component) Metadata {
	m := Metadata{
		Kind:          string(c.Kind),
		AssuranceTier: string(c.Tier),
		Coverage:      string(c.Coverage),
		CoverageNote:  coverageNote,
		Origin:        string(c.Origin),
		Dependencies:  c.Dependencies,
		FileCount:     len(c.Files),
	}
	if c.Tier == content.TierNameVersionOnly {
		m.AssuranceNote = roleAssuranceNote
	}
	return m
}

func licenses(c content.Component, loc file.Location) []pkg.License {
	out := make([]pkg.License, 0, len(c.Licenses))
	for _, l := range c.Licenses {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, pkg.NewLicenseFromLocations(l, loc))
	}
	return out
}

// purl builds the provisional identifier. Kept to one function here for the same reason the CLI
// does: when the `ansible` type is registered, the change is one edit (ADR-0004).
//
// The `kind=role` qualifier is an extension to purl-spec#854, which addresses collections and says
// nothing about roles even though Galaxy namespaces both. Without a discriminator a role and a
// collection sharing a name would collide. Raised upstream.
func purl(c content.Component) string {
	var b strings.Builder
	b.WriteString("pkg:")
	b.WriteString(Type)
	b.WriteString("/")
	b.WriteString(c.FQN())
	if c.Version != "" {
		b.WriteString("@")
		b.WriteString(c.Version)
	}
	if c.Kind == content.KindRole {
		b.WriteString("?kind=role")
	}
	return b.String()
}
