package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrjsmrtn/ansible-bom/content"
)

// buildCollection writes a minimal but genuine collection layout: files, a FILES.json recording
// their checksums, and a MANIFEST.json recording FILES.json's checksum.
func buildCollection(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	type fileEntry struct {
		Name   string `json:"name"`
		FType  string `json:"ftype"`
		Chk    string `json:"chksum_type"`
		SHA256 string `json:"chksum_sha256"`
		Format int    `json:"format"`
	}
	entries := []fileEntry{{Name: ".", FType: "dir", Format: 1}}

	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(body))
		entries = append(entries, fileEntry{
			Name: name, FType: "file", Chk: "sha256", SHA256: hex.EncodeToString(sum[:]), Format: 1,
		})
	}

	filesJSON, err := json.Marshal(map[string]any{"files": entries, "format": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FILES.json"), filesJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	fsum := sha256.Sum256(filesJSON)
	manifest := map[string]any{
		"collection_info": map[string]any{
			"namespace": "example", "name": "widget", "version": "1.0.0",
			"license": []string{"MIT"}, "dependencies": map[string]string{},
		},
		"file_manifest_file": map[string]any{
			"name": "FILES.json", "ftype": "file", "chksum_type": "sha256",
			"chksum_sha256": hex.EncodeToString(fsum[:]), "format": 1,
		},
		"format": 1,
	}
	mj, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MANIFEST.json"), mj, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func parse(t *testing.T, dir string) content.Component {
	t.Helper()
	c, err := content.ParseCollection(dir)
	if err != nil {
		t.Fatalf("ParseCollection: %v", err)
	}
	return c
}

func TestVerifyClean(t *testing.T) {
	dir := buildCollection(t, map[string]string{"plugins/a.py": "one", "README.md": "two"})
	res := Component(parse(t, dir))

	if res.Status != StatusVerified {
		t.Fatalf("Status = %q, want %q (problems: %+v)", res.Status, StatusVerified, res.Problems)
	}
	if !res.ManifestIntact {
		t.Error("ManifestIntact = false on an untouched collection")
	}
	if res.Checked != 2 {
		t.Errorf("Checked = %d, want 2", res.Checked)
	}
}

func TestVerifyDetectsModifiedFile(t *testing.T) {
	dir := buildCollection(t, map[string]string{"plugins/a.py": "one", "README.md": "two"})
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := Component(parse(t, dir))
	if res.Status != StatusModified {
		t.Fatalf("Status = %q, want %q", res.Status, StatusModified)
	}
	if len(res.Problems) != 1 || res.Problems[0].Problem != "modified" {
		t.Fatalf("Problems = %+v, want one modified file", res.Problems)
	}
	if res.Problems[0].Actual == res.Problems[0].Expected {
		t.Error("expected and actual checksums are equal on a modified file")
	}
}

func TestVerifyDetectsMissingFile(t *testing.T) {
	dir := buildCollection(t, map[string]string{"plugins/a.py": "one", "README.md": "two"})
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}

	res := Component(parse(t, dir))
	if res.Status != StatusModified {
		t.Fatalf("Status = %q, want %q", res.Status, StatusModified)
	}
	if len(res.Problems) != 1 || res.Problems[0].Problem != "missing" {
		t.Fatalf("Problems = %+v, want one missing file", res.Problems)
	}
}

// The case a naive verifier misses: an adversary edits a file and rewrites FILES.json so the
// recorded checksum matches. Every per-file comparison then passes. MANIFEST.json's record of
// FILES.json's own checksum is what catches it.
func TestVerifyDetectsRewrittenFileManifest(t *testing.T) {
	dir := buildCollection(t, map[string]string{"plugins/a.py": "one"})

	if err := os.WriteFile(filepath.Join(dir, "plugins/a.py"), []byte("malicious"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Rewrite FILES.json to match the tampered file.
	var fm map[string]any
	raw, err := os.ReadFile(filepath.Join(dir, "FILES.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fm); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("malicious"))
	for _, e := range fm["files"].([]any) {
		m := e.(map[string]any)
		if m["name"] == "plugins/a.py" {
			m["chksum_sha256"] = hex.EncodeToString(sum[:])
		}
	}
	out, err := json.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FILES.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}

	res := Component(parse(t, dir))
	if res.ManifestIntact {
		t.Error("ManifestIntact = true after FILES.json was rewritten")
	}
	if res.Status != StatusModified {
		t.Errorf("Status = %q, want %q — every file matches, but the manifest was altered",
			res.Status, StatusModified)
	}
	if !strings.Contains(res.Note, "unreliable") {
		t.Errorf("Note = %q, want it to warn that per-file results cannot be trusted", res.Note)
	}
}

// The documented limit, asserted so it cannot regress into an unnoticed false claim: an adversary
// who also rewrites MANIFEST.json defeats this check entirely, because nothing records
// MANIFEST.json's own hash. Detecting that needs the Galaxy server or a signature.
func TestVerifyCannotDetectAFullyRewrittenManifestChain(t *testing.T) {
	dir := buildCollection(t, map[string]string{"plugins/a.py": "one"})
	if err := os.WriteFile(filepath.Join(dir, "plugins/a.py"), []byte("malicious"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Rebuild both manifests around the tampered content, exactly as the installer would.
	rebuilt := buildCollection(t, map[string]string{"plugins/a.py": "malicious"})

	res := Component(parse(t, rebuilt))
	if res.Status != StatusVerified {
		t.Fatalf("Status = %q; the limit under test is that this DOES pass", res.Status)
	}
	_ = dir
}

// A role must never be reported as verified: nothing was checked, because nothing exists to
// check against (ADR-0005).
func TestRolesAreUnverifiableNeverVerified(t *testing.T) {
	res := Component(content.Component{
		Kind: content.KindRole, Name: "site_common", Tier: content.TierNameVersionOnly,
	})
	if res.Status != StatusUnverifiable {
		t.Fatalf("Status = %q, want %q", res.Status, StatusUnverifiable)
	}
	if res.Checked != 0 {
		t.Errorf("Checked = %d, want 0", res.Checked)
	}
	if !strings.Contains(res.Note, "no checksums exist") {
		t.Errorf("Note = %q, want it to attribute the gap to the ecosystem", res.Note)
	}
}

// OK() must not be read as "everything was verified" when roles were merely skipped, which is why
// Counts keeps the two separate.
func TestReportCountsSeparateVerifiedFromUnverifiable(t *testing.T) {
	dir := buildCollection(t, map[string]string{"a": "1"})
	inv := content.Inventory{Components: []content.Component{
		parse(t, dir),
		{Kind: content.KindRole, Name: "site_common", Tier: content.TierNameVersionOnly},
	}}

	rep := Inventory(inv)
	verified, modified, unverifiable, errored := rep.Counts()
	if verified != 1 || modified != 0 || unverifiable != 1 || errored != 0 {
		t.Fatalf("counts = %d/%d/%d/%d, want 1/0/1/0", verified, modified, unverifiable, errored)
	}
	if !rep.OK() {
		t.Error("OK() = false; an unverifiable component is not a failure")
	}
}
