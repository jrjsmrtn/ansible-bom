package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	inv, err := Scan(filepath.Join("testdata", "tree"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var got []string
	for _, c := range inv.Components {
		got = append(got, string(c.Kind)+":"+c.FQN())
	}

	// Collections first, then roles; each group sorted by name.
	want := []string{
		"collection:community.general",
		"collection:community.windows",
		"collection:example.widget",
		"role:jborean93.win_openssh",
		"role:site_common",
	}
	if len(got) != len(want) {
		t.Fatalf("components = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("component[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// A collection that cannot be parsed must be reported, not dropped. Silently omitting it would
// under-report the inventory, which is the failure an SBOM exists to prevent.
func TestScanReportsUnparseableCollections(t *testing.T) {
	inv, err := Scan(filepath.Join("testdata", "tree"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(inv.Problems) != 1 {
		t.Fatalf("Problems = %v, want exactly 1 (broken/future)", inv.Problems)
	}
	p := inv.Problems[0]
	if !strings.Contains(p.Path, filepath.Join("broken", "future")) {
		t.Errorf("Problem.Path = %q, want it to name broken/future", p.Path)
	}
	if !strings.Contains(p.Reason, "unsupported MANIFEST.json format 2") {
		t.Errorf("Problem.Reason = %q, want the format gate reason", p.Reason)
	}
}

// The "<ns>.<name>-<version>.info" marker directories ansible-galaxy leaves beside namespace
// directories are not collections and must not be inventoried as such.
func TestScanIgnoresInfoMarkerDirectories(t *testing.T) {
	inv, err := Scan(filepath.Join("testdata", "tree"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, c := range inv.Components {
		if strings.Contains(c.Path, ".info") {
			t.Errorf("inventoried a marker directory: %s", c.Path)
		}
	}
	for _, p := range inv.Problems {
		if strings.Contains(p.Path, ".info") {
			t.Errorf("reported a marker directory as a problem: %s", p.Path)
		}
	}
}

func TestScanCounts(t *testing.T) {
	inv, err := Scan(filepath.Join("testdata", "tree"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	collections, roles, checksummed, unversioned := inv.Counts()
	if collections != 3 {
		t.Errorf("collections = %d, want 3", collections)
	}
	if roles != 2 {
		t.Errorf("roles = %d, want 2", roles)
	}
	// Only collections are checksummed — the count must never equal the component total,
	// or a reader will take "5 components" for "5 verified" (ADR-0005).
	if checksummed != 3 {
		t.Errorf("checksummed = %d, want 3", checksummed)
	}
	if unversioned != 1 {
		t.Errorf("unversioned = %d, want 1 (the local role)", unversioned)
	}
}

// A root with neither subdirectory is a valid, empty answer rather than an error.
func TestScanEmptyRoot(t *testing.T) {
	inv, err := Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(inv.Components) != 0 || len(inv.Problems) != 0 {
		t.Errorf("empty root yielded %d components, %d problems", len(inv.Components), len(inv.Problems))
	}
}

// TestScanRealTree runs the parser against a real control node when one is offered.
//
// Fixtures are curated and therefore optimistic. Real trees contain shapes nobody anticipated,
// which is how every finding in the inception analysis was discovered. Point this at a real
// ansible_collections parent to check the parser against reality:
//
//	ANSIBLE_BOM_REAL_TREE=/path/to/content go test ./internal/content -run RealTree -v
func TestScanRealTree(t *testing.T) {
	root := os.Getenv("ANSIBLE_BOM_REAL_TREE")
	if root == "" {
		t.Skip("set ANSIBLE_BOM_REAL_TREE to a real content root to run this")
	}

	inv, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan(%s): %v", root, err)
	}

	collections, roles, checksummed, unversioned := inv.Counts()
	t.Logf("collections=%d roles=%d checksummed=%d unversioned=%d problems=%d",
		collections, roles, checksummed, unversioned, len(inv.Problems))

	for _, p := range inv.Problems {
		t.Errorf("unparsed: %s — %s", p.Path, p.Reason)
	}
	if len(inv.Components) == 0 {
		t.Errorf("no components found under %s — is it a content root?", root)
	}
	for _, c := range inv.Components {
		if c.Name == "" {
			t.Errorf("component at %s has no name", c.Path)
		}
		if c.Kind == KindCollection && len(c.Files) == 0 {
			t.Errorf("collection %s has no files — FILES.json was empty or unreadable", c.FQN())
		}
	}
}
