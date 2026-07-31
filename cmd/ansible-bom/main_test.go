package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// tree builds a content root: one collection with checksums, one Galaxy-installed role, one
// local role with no version. Self-contained, so these tests do not depend on another package's
// fixtures.
func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	coll := filepath.Join(root, "ansible_collections", "example", "widget")
	if err := os.MkdirAll(filepath.Join(coll, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "print('hello')\n"
	if err := os.WriteFile(filepath.Join(coll, "plugins", "mod.py"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(body))
	filesJSON, _ := json.Marshal(map[string]any{
		"files": []any{
			map[string]any{"name": ".", "ftype": "dir", "chksum_type": nil, "chksum_sha256": nil, "format": 1},
			map[string]any{"name": "plugins/mod.py", "ftype": "file", "chksum_type": "sha256",
				"chksum_sha256": hex.EncodeToString(sum[:]), "format": 1},
		},
		"format": 1,
	})
	if err := os.WriteFile(filepath.Join(coll, "FILES.json"), filesJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	fsum := sha256.Sum256(filesJSON)
	manifest, _ := json.Marshal(map[string]any{
		"collection_info": map[string]any{
			"namespace": "example", "name": "widget", "version": "1.0.0",
			"license": []string{"MIT"}, "dependencies": map[string]string{},
		},
		"file_manifest_file": map[string]any{
			"name": "FILES.json", "ftype": "file", "chksum_type": "sha256",
			"chksum_sha256": hex.EncodeToString(fsum[:]), "format": 1,
		},
		"format": 1,
	})
	if err := os.WriteFile(filepath.Join(coll, "MANIFEST.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	// The marker directory that makes the collection's origin identifiable as Galaxy.
	if err := os.MkdirAll(filepath.Join(root, "ansible_collections", "example.widget-1.0.0.info"), 0o755); err != nil {
		t.Fatal(err)
	}

	galaxyRole := filepath.Join(root, "roles", "author.tool", "meta")
	if err := os.MkdirAll(galaxyRole, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galaxyRole, "main.yml"), []byte("galaxy_info:\n  author: author\ndependencies: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galaxyRole, ".galaxy_install_info"), []byte("install_date: Sam  3 sep 14:26:34 2022\nversion: v2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	localRole := filepath.Join(root, "roles", "site_common", "meta")
	if err := os.MkdirAll(localRole, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localRole, "main.yml"), []byte("galaxy_info:\n  author: local\ndependencies: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func requirementsFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "requirements.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// invoke runs the CLI and returns stdout, stderr and the exit code the process would use.
func invoke(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	a := &app{stdout: &out, stderr: &errb}
	err := a.run(args)
	return out.String(), errb.String(), exitCode(err)
}

func TestUsageAndVersion(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"no arguments is a usage error", nil, 2, ""},
		{"unknown command is a usage error", []string{"frobnicate"}, 2, ""},
		{"help", []string{"help"}, 0, "inventory installed Ansible content"},
		{"version", []string{"version"}, 0, "ansible-bom " + version},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, code := invoke(t, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit = %d, want %d", code, tt.wantCode)
			}
			if tt.wantOut != "" && !strings.Contains(out, tt.wantOut) {
				t.Errorf("stdout = %q, want it to contain %q", out, tt.wantOut)
			}
		})
	}
}

// A missing root is the user's mistake, not a failure of the requested work: exit 2, not 1.
func TestMissingArgumentsExitTwo(t *testing.T) {
	for _, args := range [][]string{
		{"lock"}, {"scan"}, {"verify"},
		{"drift", "/tmp"},                   // no --requirements
		{"drift", "-r", "/nonexistent.yml"}, // no root
	} {
		if _, _, code := invoke(t, args...); code != 2 {
			t.Errorf("%v: exit = %d, want 2", args, code)
		}
	}
}

func TestLockEmitsParseableYAML(t *testing.T) {
	out, errb, code := invoke(t, "lock", tree(t))
	if code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, errb)
	}

	var lock struct {
		Version     int `yaml:"version"`
		Collections []struct {
			Name, Version, Digest string
		} `yaml:"collections"`
		Roles []struct {
			Name, Version string
		} `yaml:"roles"`
		Unpinnable []struct{ Name string } `yaml:"unpinnable"`
	}
	if err := yaml.Unmarshal([]byte(out), &lock); err != nil {
		t.Fatalf("stdout is not valid YAML: %v", err)
	}
	if lock.Version != 1 {
		t.Errorf("lockfile version = %d, want 1", lock.Version)
	}
	if len(lock.Collections) != 1 || lock.Collections[0].Name != "example.widget" {
		t.Errorf("collections = %+v", lock.Collections)
	}
	if lock.Collections[0].Digest == "" {
		t.Error("collection has no content digest")
	}
	// The v-prefix must be normalised.
	if len(lock.Roles) != 1 || lock.Roles[0].Version != "2.0.0" {
		t.Errorf("roles = %+v, want author.tool at 2.0.0", lock.Roles)
	}
	if len(lock.Unpinnable) != 1 || lock.Unpinnable[0].Name != "site_common" {
		t.Errorf("unpinnable = %+v, want site_common", lock.Unpinnable)
	}

	// The summary goes to stderr so it never contaminates piped output.
	if !strings.Contains(errb, "NOT pinned") {
		t.Errorf("stderr does not report unpinnable content: %q", errb)
	}
}

func TestLockRequirementsProjectionIsInstallable(t *testing.T) {
	out, _, code := invoke(t, "lock", "--requirements", tree(t))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}

	var req struct {
		Collections []struct{ Name, Version string } `yaml:"collections"`
		Roles       []struct{ Name, Version string } `yaml:"roles"`
	}
	if err := yaml.Unmarshal([]byte(out), &req); err != nil {
		t.Fatalf("projection is not valid YAML: %v", err)
	}
	if len(req.Collections) != 1 || len(req.Roles) != 1 {
		t.Fatalf("projection = %d collections, %d roles; want 1, 1", len(req.Collections), len(req.Roles))
	}
	// Omitted content must be named in the file itself — someone will read this without ever
	// seeing the tool's stderr.
	if !strings.Contains(out, "OMITTED") || !strings.Contains(out, "site_common") {
		t.Error("projection does not name the omitted component")
	}
}

func TestLockWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.lock.yaml")
	out, _, code := invoke(t, "lock", "-o", path, tree(t))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty when writing to a file", out)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(body), "example.widget") {
		t.Error("file does not contain the inventory")
	}
}

func TestDriftReportsAndFails(t *testing.T) {
	root := tree(t)
	req := requirementsFile(t, "collections:\n  - name: example.widget\n  - name: example.absent\n    version: 1.0.0\n")

	out, _, code := invoke(t, "drift", "-r", req, root)
	if code != 0 {
		t.Fatalf("exit = %d without --fail-on-drift", code)
	}
	for _, want := range []string{
		"Declared without an exact version", // example.widget declared bare
		"Declared but not installed",        // example.absent
		"Installed but never declared",      // author.tool
		"First-party content",               // site_common, reported as context
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n%s", want, out)
		}
	}

	// site_common must not be reported as drift.
	undeclared := out[strings.Index(out, "Installed but never declared"):]
	if i := strings.Index(undeclared, "First-party"); i > 0 {
		if strings.Contains(undeclared[:i], "site_common") {
			t.Error("a first-party role was reported as undeclared")
		}
	}

	// Nothing above breaks reproducibility: every installed component has a version and a fixed
	// source, so it can be pinned and rebuilt from a lockfile. Unpinned and undeclared
	// declarations are governance problems, not reproducibility ones.
	if _, _, code := invoke(t, "drift", "--fail-on-drift", "-r", req, root); code != 0 {
		t.Errorf("exit = %d with --fail-on-drift on a pinnable tree, want 0", code)
	}
}

// A mutable source is what actually breaks reproducibility: there is no version to pin, so a
// rebuild takes whatever the branch head is that day.
func TestDriftFailsOnMutableSource(t *testing.T) {
	root := tree(t)
	req := requirementsFile(t,
		"collections:\n  - name: git+https://example.com/org/example.widget.git\n")

	out, _, code := invoke(t, "drift", "-r", req, root)
	if !strings.Contains(out, "Tracking a moving target") {
		t.Fatalf("report does not flag the mutable source:\n%s", out)
	}
	if !strings.Contains(out, "Reproducible: NO") {
		t.Errorf("report does not state the node is irreproducible:\n%s", out)
	}
	if _, _, code2 := invoke(t, "drift", "--fail-on-drift", "-r", req, root); code2 != 1 {
		t.Errorf("exit = %d with --fail-on-drift, want 1", code2)
	}
	_ = code
}

func TestScanEmitsValidCycloneDX(t *testing.T) {
	out, errb, code := invoke(t, "scan", tree(t))
	if code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, errb)
	}

	var bom struct {
		BOMFormat   string `json:"bomFormat"`
		SpecVersion string `json:"specVersion"`
		Serial      string `json:"serialNumber"`
		Components  []struct {
			Name       string                         `json:"name"`
			PURL       string                         `json:"purl"`
			Properties []struct{ Name, Value string } `json:"properties"`
		} `json:"components"`
		Compositions []struct {
			Aggregate string `json:"aggregate"`
		} `json:"compositions"`
	}
	if err := json.Unmarshal([]byte(out), &bom); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if bom.BOMFormat != "CycloneDX" || bom.SpecVersion != "1.6" {
		t.Errorf("envelope = %s/%s", bom.BOMFormat, bom.SpecVersion)
	}
	if len(bom.Components) != 3 {
		t.Errorf("components = %d, want 3", len(bom.Components))
	}
	if bom.Compositions[0].Aggregate != "complete" {
		t.Errorf("aggregate = %q, want complete", bom.Compositions[0].Aggregate)
	}

	// Every component must carry a coverage status, and the run must say so out loud.
	for _, c := range bom.Components {
		var coverage string
		for _, p := range c.Properties {
			if p.Name == "ansible-bom:vulnerability-coverage" {
				coverage = p.Value
			}
		}
		if coverage != "not-covered" {
			t.Errorf("%s: vulnerability-coverage = %q", c.Name, coverage)
		}
	}
	if !strings.Contains(errb, "not a vulnerability assessment") {
		t.Errorf("stderr must not let 'scan' imply an assessment happened: %q", errb)
	}
	if !strings.Contains(errb, "PROVISIONAL") {
		t.Errorf("stderr does not warn that identifiers are provisional: %q", errb)
	}
}

func TestVerifyPassesThenDetectsTampering(t *testing.T) {
	root := tree(t)

	out, _, code := invoke(t, "verify", root)
	if code != 0 {
		t.Fatalf("exit = %d on a clean tree\n%s", code, out)
	}
	if !strings.Contains(out, "1 verified") {
		t.Errorf("report = %q, want 1 verified", out)
	}
	// A role must be reported as unverifiable, and that must not read as a pass.
	if !strings.Contains(out, "unverifiable is not a pass") {
		t.Error("report does not warn that unverifiable is not a pass")
	}

	mod := filepath.Join(root, "ansible_collections", "example", "widget", "plugins", "mod.py")
	if err := os.WriteFile(mod, []byte("print('tampered')\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, code = invoke(t, "verify", root)
	if code != 1 {
		t.Errorf("exit = %d after tampering, want 1", code)
	}
	if !strings.Contains(out, "FAILED") || !strings.Contains(out, "modified") {
		t.Errorf("report does not name the modification:\n%s", out)
	}

	// --exit-zero reports the failure but does not fail the process.
	out, _, code = invoke(t, "verify", "--exit-zero", root)
	if code != 0 {
		t.Errorf("exit = %d with --exit-zero, want 0", code)
	}
	if !strings.Contains(out, "FAILED") {
		t.Error("--exit-zero suppressed the report as well as the exit code")
	}
}

// Content that cannot be parsed must be reported, and --fail-on-problems must act on it.
func TestFailOnProblems(t *testing.T) {
	root := tree(t)
	broken := filepath.Join(root, "ansible_collections", "broken", "future")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"MANIFEST.json", "FILES.json"} {
		if err := os.WriteFile(filepath.Join(broken, f), []byte(`{"format": 99}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, errb, code := invoke(t, "lock", root)
	if code != 0 {
		t.Errorf("exit = %d without the flag, want 0", code)
	}
	if !strings.Contains(errb, "NOT inventoried") {
		t.Errorf("stderr does not report unparsed content: %q", errb)
	}

	if _, _, code := invoke(t, "lock", "--fail-on-problems", root); code != 1 {
		t.Errorf("exit = %d with --fail-on-problems, want 1", code)
	}

	// The BOM must declare itself incomplete rather than claim to be exhaustive.
	out, _, _ := invoke(t, "scan", root)
	if !strings.Contains(out, `"aggregate": "incomplete"`) {
		t.Error("BOM does not declare itself incomplete when content could not be parsed")
	}
}
