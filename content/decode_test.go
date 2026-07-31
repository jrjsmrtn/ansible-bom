package content

import (
	"strings"
	"testing"
)

// The decoders are the public, reader-based contract this package exists to offer — a syft parser
// is handed a reader, never a directory. Until now they were only exercised transitively through
// ParseCollection and ParseRole, which means the boundary that consumers actually call had no
// tests of its own. These cover it directly.

func TestDecodeManifest(t *testing.T) {
	const valid = `{
	  "collection_info": {
	    "namespace": "community", "name": "general", "version": "11.4.0",
	    "license": ["GPL-3.0-or-later"],
	    "dependencies": {"ansible.posix": ">=1.0.0"},
	    "repository": "https://www.github.com/my_org/my_collection"
	  },
	  "file_manifest_file": {"name": "FILES.json", "chksum_sha256": "abc123"},
	  "format": 1
	}`

	m, err := DecodeManifest(strings.NewReader(valid))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if m.Namespace != "community" || m.Name != "general" || m.Version != "11.4.0" {
		t.Errorf("identity = %s.%s@%s", m.Namespace, m.Name, m.Version)
	}
	if m.FilesDigest != "abc123" {
		t.Errorf("FilesDigest = %q — verify cannot check the file manifest without it", m.FilesDigest)
	}
	if m.Dependencies["ansible.posix"] != ">=1.0.0" {
		t.Errorf("Dependencies = %v; constraints are recorded verbatim, never resolved", m.Dependencies)
	}
	// The placeholder `repository` must not have been promoted into identity (ADR-0002 §5).
	if strings.Contains(m.Namespace+m.Name, "my_org") {
		t.Error("an untrusted manifest field leaked into identity")
	}
}

func TestDecodeManifestRejects(t *testing.T) {
	tests := []struct {
		name, input, wantErr string
	}{
		{
			// The ADR-0007 gate: no schema exists for this file, so an unrecognised shape is
			// declined rather than guessed at.
			name:    "unsupported format",
			input:   `{"collection_info":{"namespace":"a","name":"b"},"format":2}`,
			wantErr: "unsupported MANIFEST.json format 2",
		},
		{
			name:    "no namespace or name",
			input:   `{"collection_info":{"version":"1.0.0"},"format":1}`,
			wantErr: "declares no namespace/name",
		},
		{
			name:    "not JSON",
			input:   `not json at all`,
			wantErr: "parsing MANIFEST.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeManifest(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeFiles(t *testing.T) {
	const input = `{"files":[
	  {"name":"z.py","ftype":"file","chksum_sha256":"zzz"},
	  {"name":".","ftype":"dir","chksum_sha256":null},
	  {"name":"a.py","ftype":"file","chksum_sha256":"aaa"}
	],"format":1}`

	files, err := DecodeFiles(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2 — directory entries are not content", len(files))
	}
	// Sorted, so output is diffable between runs.
	if files[0].Path != "a.py" || files[1].Path != "z.py" {
		t.Errorf("not sorted: %+v", files)
	}
	for _, f := range files {
		if f.SHA256 == "" {
			t.Errorf("%s has no checksum — a directory entry leaked through", f.Path)
		}
	}

	if _, err := DecodeFiles(strings.NewReader(`{"files":[],"format":9}`)); err == nil {
		t.Error("an unrecognised FILES.json format was accepted")
	}
}

func TestDecodeRoleMeta(t *testing.T) {
	const input = `
galaxy_info:
  namespace: example
  role_name: composite
  version: 2.1.0
dependencies:
  - example.base
  - role: example.database
    version: 1.4.0
  - name: example.cache
    version: "0.9"
collections:
  - community.general
unknown_key_no_schema_anticipates: true
`
	m, err := DecodeRoleMeta(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeRoleMeta: %v", err)
	}
	if m.Namespace != "example" || m.RoleName != "composite" || m.Version != "2.1.0" {
		t.Errorf("identity = %s.%s@%s", m.Namespace, m.RoleName, m.Version)
	}
	want := map[string]string{
		"example.base": "", "example.database": "1.4.0",
		"example.cache": "0.9", "community.general": "",
	}
	if len(m.Dependencies) != len(want) {
		t.Fatalf("Dependencies = %v, want %v", m.Dependencies, want)
	}
	for k, v := range want {
		if m.Dependencies[k] != v {
			t.Errorf("Dependencies[%q] = %q, want %q", k, m.Dependencies[k], v)
		}
	}
}

// Role metadata in the wild carries keys no schema anticipates. Rejecting them would make the
// tool useless on the estates that most need it — it inventories, it does not lint (ADR-0007).
func TestDecodeRoleMetaToleratesUnknownKeys(t *testing.T) {
	if _, err := DecodeRoleMeta(strings.NewReader("galaxy_info:\n  author: x\nwibble: 3\n")); err != nil {
		t.Fatalf("rejected metadata ansible-core would accept: %v", err)
	}
}

func TestDecodeInstallInfo(t *testing.T) {
	// install_date is locale-formatted and must never be parsed; only the version is returned.
	v, ok := DecodeInstallInfo(strings.NewReader("install_date: Mer  1 déc 16:22:06 2021\nversion: v1.1.3\n"))
	if !ok {
		t.Fatal("a valid install info was rejected")
	}
	if v != "v1.1.3" {
		t.Errorf("version = %q — the decoder returns it verbatim; normalisation happens later", v)
	}

	for _, in := range []string{"", "install_date: whenever\n", "not: yaml: at: all: ["} {
		if _, ok := DecodeInstallInfo(strings.NewReader(in)); ok {
			t.Errorf("accepted install info with no version: %q", in)
		}
	}
}

// ComponentFromRoleMeta carries the identity precedence: the directory name wins, because role
// metadata frequently omits namespace and role_name.
func TestComponentFromRoleMeta(t *testing.T) {
	meta := RoleMeta{Namespace: "meta", RoleName: "declared", Version: "9.9.9"}

	c := ComponentFromRoleMeta("author.tool", meta, "v2.0.0")
	if c.FQN() != "author.tool" {
		t.Errorf("FQN = %q, want the directory name to win", c.FQN())
	}
	if c.Version != "2.0.0" {
		t.Errorf("Version = %q, want the installer's record, v-prefix normalised", c.Version)
	}
	if c.Origin != OriginGalaxy {
		t.Errorf("Origin = %q, want galaxy when an installer recorded a version", c.Origin)
	}

	// No installer record: fall back to the author's declaration, and to metadata identity when
	// the directory name carries no namespace.
	c = ComponentFromRoleMeta("plain", meta, "")
	if c.FQN() != "meta.declared" {
		t.Errorf("FQN = %q, want the metadata fallback", c.FQN())
	}
	if c.Version != "9.9.9" || c.Origin != OriginLocal {
		t.Errorf("version/origin = %q/%q", c.Version, c.Origin)
	}

	// Nothing anywhere: unversioned, never a placeholder.
	c = ComponentFromRoleMeta("orphan", RoleMeta{}, "")
	if c.Versioned() {
		t.Errorf("Version = %q, want empty", c.Version)
	}
	if c.Tier != TierNameVersionOnly {
		t.Errorf("Tier = %q — roles are never checksummed", c.Tier)
	}
}
