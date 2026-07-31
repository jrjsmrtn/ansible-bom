package requirements

import "testing"

func TestParseBytesModernForm(t *testing.T) {
	src := []byte(`
collections:
  - name: community.general
  - name: community.windows
    version: ">=3.0.0,<4.0.0"
  - ansible.posix
  - name: git+file:///srv/src/example.widget/
roles:
  - src: geerlingguy.postgresql
    version: 3.5.0
  - name: alvistack.cri_o
  - jborean93.win_openssh
`)
	f, err := ParseBytes(src)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if len(f.Collections) != 4 {
		t.Fatalf("collections = %d, want 4", len(f.Collections))
	}
	if len(f.Roles) != 3 {
		t.Fatalf("roles = %d, want 3", len(f.Roles))
	}
	if f.Collections[2].Name != "ansible.posix" {
		t.Errorf("bare string entry = %q, want ansible.posix", f.Collections[2].Name)
	}
	if f.Roles[0].Version != "3.5.0" {
		t.Errorf("role src form lost its version: %q", f.Roles[0].Version)
	}
}

// The legacy form is a bare sequence of roles with no section keys.
func TestParseBytesLegacyRoleList(t *testing.T) {
	f, err := ParseBytes([]byte("- src: geerlingguy.postgresql\n  version: 3.5.0\n- jborean93.win_openssh\n"))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if len(f.Roles) != 2 || len(f.Collections) != 0 {
		t.Fatalf("legacy form = %d roles, %d collections; want 2, 0", len(f.Roles), len(f.Collections))
	}
}

func TestFQN(t *testing.T) {
	tests := []struct {
		name        string
		decl        Declaration
		wantFQN     string
		wantDerived bool
	}{
		{"plain name", Declaration{Name: "community.general"}, "community.general", false},
		{"git+file url", Declaration{Name: "git+file:///srv/src/example.widget/"}, "example.widget", true},
		{"git https with .git", Declaration{Name: "git+https://example.com/org/example.widget.git"}, "example.widget", true},
		{"git url with ansible's comma version", Declaration{Name: "git+https://example.com/org/example.widget.git,1.2.3"}, "example.widget", true},
		{"scp-style", Declaration{Name: "git@example.com:org/example.widget.git"}, "example.widget", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decl.FQN(); got != tt.wantFQN {
				t.Errorf("FQN = %q, want %q", got, tt.wantFQN)
			}
			if got := tt.decl.IsDerived(); got != tt.wantDerived {
				t.Errorf("IsDerived = %v, want %v", got, tt.wantDerived)
			}
		})
	}
}

// A range is not a pin. This is the distinction the whole tool turns on: ">=1.0.0" and "*" both
// accept whatever the server offers at install time.
func TestPinned(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"3.5.0", true},
		{"v1.2.3", true},
		{"", false},
		{"*", false},
		{">=3.0.0,<4.0.0", false},
		{">=1.0.0", false},
		{"~1.2", false},
		{"1.0.0|2.0.0", false},
	}
	for _, tt := range tests {
		if got := (Declaration{Version: tt.version}).Pinned(); got != tt.want {
			t.Errorf("Pinned(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestMutable(t *testing.T) {
	tests := []struct {
		name string
		decl Declaration
		want bool
	}{
		{"git url with no version tracks a branch", Declaration{Name: "git+https://example.com/o/r.git"}, true},
		{"git url with a version is fixed", Declaration{Name: "git+https://example.com/o/r.git", Version: "1.0.0"}, false},
		{"type git with no version", Declaration{Name: "example.widget", Type: "git"}, true},
		{"scm git with no version", Declaration{Name: "example.widget", SCM: "git"}, true},
		{"plain galaxy name is not mutable, only unpinned", Declaration{Name: "community.general"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decl.Mutable(); got != tt.want {
				t.Errorf("Mutable = %v, want %v", got, tt.want)
			}
		})
	}
}

// Unexpected keys must not cause a failure: this tool inventories, it does not lint (ADR-0007).
func TestParseBytesToleratesUnknownKeys(t *testing.T) {
	f, err := ParseBytes([]byte("collections:\n  - name: community.general\n    signatures: [foo]\n    unknown_key: 1\n"))
	if err != nil {
		t.Fatalf("ParseBytes rejected a file ansible-galaxy would accept: %v", err)
	}
	if len(f.Collections) != 1 || f.Collections[0].Name != "community.general" {
		t.Errorf("collections = %+v", f.Collections)
	}
}
