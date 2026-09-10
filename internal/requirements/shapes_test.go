// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package requirements

import (
	"os"
	"path/filepath"
	"testing"
)

// This table is the executable counterpart to docs/reference/requirements-formats.md. The
// reference says what the formats are; this says what this parser does with each one.
//
// Every row is now believed FAITHFUL to ansible-core 2.20.0. That was not true when this table
// was written: nine rows recorded behaviour known to be wrong, each naming the issue that would
// fix it, and a companion test refused a divergence without an issue behind it. Issues #13, #14,
// #15 and #20 emptied the list, and that test deleted itself with the last one, as its own
// message instructed.
//
// If a divergence is found again, record it the same way — assert what the parser DOES, name the
// issue, and let the fix flip the row. Asserting the correct-in-principle value leaves the suite
// red for work nobody has started; asserting nothing leaves the divergence undetectable, which is
// how the first nine survived.
//
// Shapes come from the schema bundled with ansible-lint plus the three functions that decide
// meaning: GalaxyCLI._parse_requirements_file, RoleRequirement.role_yaml_parse and
// RoleRequirement.repo_url_to_role_name.

type want struct {
	kind    Kind
	name    string
	fqn     string
	version string
	source  string
	typ     string
	scm     string
	derived bool
}

type shapeCase struct {
	name string
	doc  string
	// want is what the parser produces, in order. Empty means the parser yields nothing.
	want []want
	// wantErr is true where ParseBytes must fail rather than return a partial answer.
	wantErr bool
	// wantUnread is content the file references that the parser deliberately did not read.
	wantUnread int
}

func shapeCases() []shapeCase {
	return []shapeCase{
		// ---- collections -------------------------------------------------------------
		{
			name: "collection/bare string",
			doc:  "collections:\n  - community.general\n",
			want: []want{{kind: KindCollection, name: "community.general", fqn: "community.general"}},
		},
		{
			name: "collection/name only",
			doc:  "collections:\n  - name: community.general\n",
			want: []want{{kind: KindCollection, name: "community.general", fqn: "community.general"}},
		},
		{
			name: "collection/name and version",
			doc:  "collections:\n  - name: community.general\n    version: 1.0.0\n",
			want: []want{{kind: KindCollection, name: "community.general", fqn: "community.general", version: "1.0.0"}},
		},
		{
			name: "collection/source and type galaxy",
			doc:  "collections:\n  - name: c.g\n    source: https://galaxy.ansible.com\n    type: galaxy\n",
			want: []want{{kind: KindCollection, name: "c.g", fqn: "c.g", source: "https://galaxy.ansible.com", typ: "galaxy"}},
		},
		{
			name: "collection/type git with version",
			doc:  "collections:\n  - name: https://github.com/o/r.git\n    type: git\n    version: main\n",
			want: []want{{kind: KindCollection, fqn: "", version: "main", source: "https://github.com/o/r.git", typ: "git"}},
		},
		{
			name: "collection/git URL as name",
			doc:  "collections:\n  - name: git+file:///srv/src/example.widget/\n",
			want: []want{{kind: KindCollection, fqn: "", source: "git+file:///srv/src/example.widget/"}},
		},
		{
			name: "collection/git URL with comma version",
			doc:  "collections:\n  - name: https://github.com/o/r.git,v1.2.3\n",
			// ansible's repo_url_to_role_name strips .git BEFORE splitting on the comma, so the
			// .git is no longer terminal and survives: it derives "r.git", not "r".
			want: []want{{kind: KindCollection, fqn: "", source: "https://github.com/o/r.git,v1.2.3"}},
		},
		{
			name: "collection/type file names a path",
			doc:  "collections:\n  - name: /tmp/c.tar.gz\n    type: file\n",
			// A path is not an identity. It can never match an installed namespace.name, and
			// derived=false tells a caller nothing is wrong.
			want: []want{{kind: KindCollection, fqn: "", source: "/tmp/c.tar.gz", typ: "file"}},
		},
		{
			name: "collection/tarball URL keeps its suffix",
			doc:  "collections:\n  - name: http://x/role.tar.gz\n",
			want: []want{{kind: KindCollection, fqn: "", source: "http://x/role.tar.gz"}},
		},
		{
			name: "collection/URL with trailing slash",
			doc:  "collections:\n  - name: http://x/repo/\n",
			want: []want{{kind: KindCollection, fqn: "", source: "http://x/repo/"}},
		},

		{
			name: "collection/namespace that is a Python keyword",
			doc:  "collections:\n  - name: if.name\n",
			// ansible rejects this: is_valid_collection_name requires each half to be a
			// non-keyword identifier, so the name is unidentifiable and the file's string is
			// kept as the source.
			want: []want{{kind: KindCollection, fqn: "", source: "if.name"}},
		},
		{
			name: "collection/non-ASCII namespace",
			doc:  "collections:\n  - name: ünï.çôdé\n",
			// Unicode letters are accepted by both now. The residual divergence is not here: it
			// is in the 4666 code points where Go's ID_Start and Python's XID_Start disagree,
			// which no corpus of plausible names reaches. See isIdentifier.
			want: []want{{kind: KindCollection, name: "ünï.çôdé", fqn: "ünï.çôdé"}},
		},

		// ---- roles -------------------------------------------------------------------
		{
			name: "role/bare string",
			doc:  "roles:\n  - jborean93.win_openssh\n",
			want: []want{{kind: KindRole, name: "jborean93.win_openssh", fqn: "jborean93.win_openssh"}},
		},
		{
			name: "role/src only",
			doc:  "roles:\n  - src: geerlingguy.postgresql\n",
			want: []want{{kind: KindRole, name: "geerlingguy.postgresql", fqn: "geerlingguy.postgresql", source: "geerlingguy.postgresql"}},
		},
		{
			name: "role/src and version",
			doc:  "roles:\n  - src: geerlingguy.postgresql\n    version: 3.5.0\n",
			want: []want{{kind: KindRole, name: "geerlingguy.postgresql", fqn: "geerlingguy.postgresql", version: "3.5.0", source: "geerlingguy.postgresql"}},
		},
		{
			name: "role/src with name is install-as",
			doc:  "roles:\n  - src: https://github.com/o/r\n    name: myrole\n",
			// name wins over src, which is right: ansible installs it under name.
			want: []want{{kind: KindRole, name: "myrole", fqn: "myrole", source: "https://github.com/o/r"}},
		},
		{
			name: "role/scm git",
			doc:  "roles:\n  - src: https://github.com/o/r\n    scm: git\n    version: main\n",
			want: []want{{kind: KindRole, name: "https://github.com/o/r", fqn: "r", version: "main", source: "https://github.com/o/r", scm: "git", derived: true}},
		},
		{
			name: "role/scm hg",
			doc:  "roles:\n  - src: https://hg.example/r\n    scm: hg\n",
			want: []want{{kind: KindRole, name: "https://hg.example/r", fqn: "r", source: "https://hg.example/r", scm: "hg", derived: true}},
		},
		{
			name: "role/bare string with comma version",
			doc:  "roles:\n  - geerlingguy.postgresql,3.5.0\n",
			// role_yaml_parse splits src[,version[,name]] for a bare STRING role. This is the one
			// path where the comma really is ansible's separator, and the one we do not implement.
			want: []want{{kind: KindRole, name: "geerlingguy.postgresql", fqn: "geerlingguy.postgresql", version: "3.5.0", source: "geerlingguy.postgresql"}},
		},
		{
			name: "role/old-style role key",
			doc:  "roles:\n  - role: legacy.alias\n",
			// `role:` is an alias for `name` and is in ansible's VALID_SPEC_KEYS.
			want: []want{{kind: KindRole, name: "legacy.alias", fqn: "legacy.alias"}},
		},

		{
			name: "role/three-field bare string",
			doc:  "roles:\n  - geerlingguy.postgresql,3.5.0,pgrole\n",
			// src,version,name — the third field is the install-as name, so it is the identity.
			want: []want{{kind: KindRole, name: "pgrole", fqn: "pgrole", version: "3.5.0", source: "geerlingguy.postgresql"}},
		},
		{
			name: "role/too many commas is left unsplit",
			doc:  "roles:\n  - a,b,c,d\n",
			// AnsibleError there. Recorded whole rather than failing the file, so it matches
			// nothing installed.
			want: []want{{kind: KindRole, name: "a,b,c,d", fqn: "a,b,c,d"}},
		},
		{
			name: "role/comma after .git survives the strip order",
			doc:  "roles:\n  - src: https://github.com/o/r.git,v1.2.3\n",
			// repo_url_to_role_name strips .git BEFORE the comma split, so .git is not terminal
			// when that check runs and survives into the name.
			want: []want{{kind: KindRole, name: "https://github.com/o/r.git,v1.2.3", fqn: "r.git", source: "https://github.com/o/r.git,v1.2.3", derived: true}},
		},
		{
			name: "role/tarball URL loses its suffix",
			doc:  "roles:\n  - src: http://x/role.tar.gz\n",
			want: []want{{kind: KindRole, name: "http://x/role.tar.gz", fqn: "role", source: "http://x/role.tar.gz", derived: true}},
		},
		{
			name: "role/trailing slash derives an empty name",
			doc:  "roles:\n  - src: http://x/repo/\n",
			// No trailing-slash trim upstream, so the last segment is empty. Reproduced.
			want: []want{{kind: KindRole, name: "http://x/repo/", fqn: "", source: "http://x/repo/", derived: true}},
		},
		{
			name: "role/scp-style remote is a URL",
			doc:  "roles:\n  - src: git@host:path/r.git\n",
			// ansible's test is `'://' in s or '@' in s`, which catches this form.
			want: []want{{kind: KindRole, name: "git@host:path/r.git", fqn: "r", source: "git@host:path/r.git", derived: true}},
		},

		// ---- structure ---------------------------------------------------------------
		{
			name: "structure/legacy bare list",
			doc:  "- src: geerlingguy.postgresql\n  version: 3.5.0\n",
			want: []want{{kind: KindRole, name: "geerlingguy.postgresql", fqn: "geerlingguy.postgresql", version: "3.5.0", source: "geerlingguy.postgresql"}},
		},
		{
			name:       "structure/legacy include",
			doc:        "- include: more.yml\n",
			want:       nil,
			wantUnread: 1,
		},
		{
			name:    "structure/empty document",
			doc:     "",
			wantErr: true,
		},
		{
			name: "structure/null section",
			doc:  "collections:\n",
			want: nil,
		},
		{
			name: "structure/both sections",
			doc:  "collections:\n  - c.g\nroles:\n  - r.n\n",
			want: []want{
				{kind: KindCollection, name: "c.g", fqn: "c.g"},
				{kind: KindRole, name: "r.n", fqn: "r.n"},
			},
		},
		{
			name: "structure/unknown keys on an entry are tolerated",
			doc:  "collections:\n  - name: c.g\n    signatures: [x]\n    zzz: 1\n",
			want: []want{{kind: KindCollection, name: "c.g", fqn: "c.g"}},
		},
		{
			name:    "structure/unknown top-level key",
			doc:     "collection:\n  - c.g\n",
			wantErr: true,
		},
		{
			name:    "structure/scalar where a section belongs",
			doc:     "collections: nope\n",
			wantErr: true,
		},
	}
}

// TestShapes asserts what the parser makes of every shape the format admits.
func TestShapes(t *testing.T) {
	for _, tc := range shapeCases() {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseBytes([]byte(tc.doc))
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}

			if len(f.Unread) != tc.wantUnread {
				t.Errorf("Unread = %d, want %d: %+v", len(f.Unread), tc.wantUnread, f.Unread)
			}

			got := append(append([]Declaration{}, f.Collections...), f.Roles...)
			if len(got) != len(tc.want) {
				t.Fatalf("entries = %d, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				g := got[i]
				for _, f := range []struct {
					field     string
					got, want any
				}{
					{"Kind", g.Kind, w.kind},
					{"Name", g.Name, w.name},
					{"FQN()", g.FQN(), w.fqn},
					{"Version", g.Version, w.version},
					{"Source", g.Source, w.source},
					{"Type", g.Type, w.typ},
					{"SCM", g.SCM, w.scm},
					{"IsDerived()", g.IsDerived(), w.derived},
				} {
					if f.got != f.want {
						t.Errorf("entry %d %s = %v, want %v", i, f.field, f.got, f.want)
					}
				}
			}
		})
	}
}

// Parse is the file-IO wrapper around ParseBytes and was the only uncovered function here.
func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.yml")
	if err := os.WriteFile(path, []byte("collections:\n  - community.general\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	f, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Collections) != 1 || f.Collections[0].Name != "community.general" {
		t.Errorf("Parse = %+v, want one community.general entry", f.Collections)
	}

	// A missing file must fail rather than read as a file declaring nothing — the two are
	// opposite answers for drift, which compares against whatever it was given.
	if _, err := Parse(filepath.Join(dir, "absent.yml")); err == nil {
		t.Error("Parse on a missing file returned no error")
	}
}

// Issue #15. Two file-level faults make ansible refuse the file outright, so a declaration set
// derived from one answers a question nobody asked: every installed component reads as
// undeclared. Rejecting matches ansible and, more importantly, is the difference between "you
// declared nothing" and "this file does not install".
func TestFileLevelFaultsAreRejected(t *testing.T) {
	for name, doc := range map[string]string{
		"empty document":                "",
		"only whitespace":               "\n\n",
		"typo'd section":                "collection:\n  - c.g\n",
		"unknown key beside a good one": "roles:\n  - r.n\ncollectons:\n  - c.g\n",
	} {
		if _, err := ParseBytes([]byte(doc)); err == nil {
			t.Errorf("%s: accepted a file ansible-galaxy refuses", name)
		}
	}

	// Entry-level tolerance is unaffected — ADR-0007, and ansible is asymmetric the same way.
	if _, err := ParseBytes([]byte("collections:\n  - name: c.g\n    zzz: 1\n")); err != nil {
		t.Errorf("unknown key on an ENTRY must stay tolerated: %v", err)
	}
	// A section present but empty is not the same as no sections at all.
	if _, err := ParseBytes([]byte("collections:\n")); err != nil {
		t.Errorf("a null section must not be rejected: %v", err)
	}
}

// An include names content this tool did not read. Recording it is the whole fix: a short
// declaration set silently reports installed content as undeclared.
func TestIncludeIsRecordedNotDropped(t *testing.T) {
	f, err := ParseBytes([]byte("- include: more.yml\n- src: geerlingguy.postgresql\n  version: 3.5.0\n"))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if len(f.Roles) != 1 {
		t.Errorf("roles = %d, want 1 (the include must not swallow the entry after it)", len(f.Roles))
	}
	if len(f.Unread) != 1 {
		t.Fatalf("Unread = %+v, want one entry", f.Unread)
	}
	if f.Unread[0].Ref != "more.yml" {
		t.Errorf("Unread ref = %q, want more.yml", f.Unread[0].Ref)
	}
	if f.Unread[0].Reason == "" {
		t.Error("an unread reference with no reason tells a reader nothing")
	}
}
