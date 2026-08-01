// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package cataloger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/source/directorysource"
)

// tree writes a content root: one collection with checksums, one Galaxy-installed role whose
// version is v-prefixed, and one local role with no version at all.
func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	coll := filepath.Join(root, "ansible_collections", "example", "widget")
	if err := os.MkdirAll(filepath.Join(coll, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "print('hello')\n"
	write(t, filepath.Join(coll, "plugins", "mod.py"), body)

	sum := sha256.Sum256([]byte(body))
	filesJSON, _ := json.Marshal(map[string]any{
		"files": []any{
			map[string]any{"name": ".", "ftype": "dir", "chksum_type": nil, "chksum_sha256": nil, "format": 1},
			map[string]any{"name": "plugins/mod.py", "ftype": "file", "chksum_type": "sha256",
				"chksum_sha256": hex.EncodeToString(sum[:]), "format": 1},
		},
		"format": 1,
	})
	write(t, filepath.Join(coll, "FILES.json"), string(filesJSON))

	fsum := sha256.Sum256(filesJSON)
	manifest, _ := json.Marshal(map[string]any{
		"collection_info": map[string]any{
			"namespace": "example", "name": "widget", "version": "1.0.0",
			"license": []string{"MIT"}, "dependencies": map[string]string{"other.thing": ">=1.0.0"},
			// A real collection was observed shipping this unedited skeleton placeholder.
			"repository": "https://www.github.com/my_org/my_collection",
		},
		"file_manifest_file": map[string]any{
			"name": "FILES.json", "ftype": "file", "chksum_type": "sha256",
			"chksum_sha256": hex.EncodeToString(fsum[:]), "format": 1,
		},
		"format": 1,
	})
	write(t, filepath.Join(coll, "MANIFEST.json"), string(manifest))

	galaxyMeta := filepath.Join(root, "roles", "author.tool", "meta")
	if err := os.MkdirAll(galaxyMeta, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(galaxyMeta, "main.yml"), "galaxy_info:\n  author: author\ndependencies: []\n")
	write(t, filepath.Join(galaxyMeta, ".galaxy_install_info"),
		"install_date: Sam  3 sep 14:26:34 2022\nversion: v2.0.0\n")

	localMeta := filepath.Join(root, "roles", "site_common", "meta")
	if err := os.MkdirAll(localMeta, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(localMeta, "main.yml"), "galaxy_info:\n  author: local\ndependencies: []\n")

	return root
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// catalog runs a cataloger through syft's real directory resolver rather than a mock, so the
// globs are exercised as syft would apply them.
func catalog(t *testing.T, c pkg.Cataloger, root string) []pkg.Package {
	t.Helper()
	src, err := directorysource.NewFromPath(root)
	if err != nil {
		t.Fatalf("directorysource: %v", err)
	}
	resolver, err := src.FileResolver("squashed")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	pkgs, _, err := c.Catalog(context.Background(), resolver)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return pkgs
}

func TestCollectionCataloger(t *testing.T) {
	pkgs := catalog(t, NewCollectionCataloger(), tree(t))
	if len(pkgs) != 1 {
		t.Fatalf("packages = %d, want 1: %+v", len(pkgs), pkgs)
	}
	p := pkgs[0]

	if p.Name != "example.widget" {
		t.Errorf("Name = %q, want example.widget", p.Name)
	}
	if p.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", p.Version)
	}
	if p.PURL != "pkg:ansible/example/widget@1.0.0" {
		t.Errorf("PURL = %q", p.PURL)
	}

	m, ok := p.Metadata.(Metadata)
	if !ok {
		t.Fatalf("Metadata is %T, want cataloger.Metadata", p.Metadata)
	}
	if m.AssuranceTier != "checksummed" {
		t.Errorf("AssuranceTier = %q, want checksummed", m.AssuranceTier)
	}
	// FILES.json must have been resolved through the resolver, not assumed absent.
	if m.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 — FILES.json was not read via the resolver", m.FileCount)
	}
	// Declared constraints are recorded verbatim, never resolved.
	if m.Dependencies["other.thing"] != ">=1.0.0" {
		t.Errorf("Dependencies = %v", m.Dependencies)
	}
	if m.CoverageNote == "" {
		t.Error("no coverage note — an empty scanner result must not read as clean")
	}
}

func TestRoleCataloger(t *testing.T) {
	pkgs := catalog(t, NewRoleCataloger(), tree(t))
	if len(pkgs) != 2 {
		t.Fatalf("packages = %d, want 2", len(pkgs))
	}

	byName := map[string]pkg.Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}

	galaxy, ok := byName["author.tool"]
	if !ok {
		t.Fatalf("author.tool not catalogued: %v", byName)
	}
	// The v prefix must be normalised, and the version must come from the install info.
	if galaxy.Version != "2.0.0" {
		t.Errorf("author.tool version = %q, want 2.0.0", galaxy.Version)
	}
	if galaxy.PURL != "pkg:ansible/author/tool@2.0.0?kind=role" {
		t.Errorf("author.tool PURL = %q — roles need the kind qualifier to avoid colliding "+
			"with a collection of the same name", galaxy.PURL)
	}

	local, ok := byName["site_common"]
	if !ok {
		t.Fatalf("site_common not catalogued")
	}
	if local.Version != "" {
		t.Errorf("site_common version = %q, want empty — no version is recorded on disk", local.Version)
	}
	if local.PURL != "pkg:ansible/site_common?kind=role" {
		t.Errorf("site_common PURL = %q", local.PURL)
	}
}

// Roles must never be catalogued as if they carried integrity data, and must say why (ADR-0005).
func TestRolesDeclareTheAbsenceOfChecksums(t *testing.T) {
	for _, p := range catalog(t, NewRoleCataloger(), tree(t)) {
		m, ok := p.Metadata.(Metadata)
		if !ok {
			t.Fatalf("%s: Metadata is %T", p.Name, p.Metadata)
		}
		if m.AssuranceTier != "name-version-only" {
			t.Errorf("%s: AssuranceTier = %q", p.Name, m.AssuranceTier)
		}
		if m.FileCount != 0 {
			t.Errorf("%s: FileCount = %d, want 0", p.Name, m.FileCount)
		}
		if m.AssuranceNote == "" {
			t.Errorf("%s: no assurance note — absence of checksums must not read as a scan failure", p.Name)
		}
	}
}

// The catalogers must not cross-match: a role tree is not a collection tree.
func TestCatalogersDoNotOverlap(t *testing.T) {
	root := tree(t)
	for _, tc := range []struct {
		name string
		c    pkg.Cataloger
		want int
	}{
		{CollectionCatalogerName, NewCollectionCataloger(), 1},
		{RoleCatalogerName, NewRoleCataloger(), 2},
	} {
		if got := len(catalog(t, tc.c, root)); got != tc.want {
			t.Errorf("%s catalogued %d package(s), want %d", tc.name, got, tc.want)
		}
	}
}

func TestCatalogerNamesFollowSyftConvention(t *testing.T) {
	for _, c := range []pkg.Cataloger{NewCollectionCataloger(), NewRoleCataloger()} {
		if got := c.Name(); got != CollectionCatalogerName && got != RoleCatalogerName {
			t.Errorf("unexpected cataloger name %q", got)
		}
	}
}

// An empty tree yields nothing rather than failing: a control node with no content is a valid
// answer, not an error.
func TestEmptyTree(t *testing.T) {
	root := t.TempDir()
	if got := len(catalog(t, NewCollectionCataloger(), root)); got != 0 {
		t.Errorf("collections = %d, want 0", got)
	}
	if got := len(catalog(t, NewRoleCataloger(), root)); got != 0 {
		t.Errorf("roles = %d, want 0", got)
	}
}
