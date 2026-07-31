package content

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestParseRole(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		wantFQN     string
		wantVersion string
		wantOrigin  Origin
		wantDeps    map[string]string
	}{
		{
			// The version is v-prefixed on disk; role versions are whatever the author tagged.
			name:        "galaxy installed role, v-prefixed version",
			dir:         "role-galaxy",
			wantFQN:     "role-galaxy",
			wantVersion: "0.3.2",
			wantOrigin:  OriginGalaxy,
			wantDeps:    map[string]string{},
		},
		{
			// No install metadata: unversioned, and never defaulted to a placeholder.
			name:        "locally authored role has no version",
			dir:         "role-local",
			wantFQN:     "role-local",
			wantVersion: "",
			wantOrigin:  OriginLocal,
			wantDeps:    map[string]string{},
		},
		{
			// Two behaviours at once: the author-declared version is used when no installer
			// recorded one, and identity falls back to galaxy_info when the directory name
			// carries no namespace.
			name:        "author-declared version and metadata identity fallback",
			dir:         "role-with-deps",
			wantFQN:     "example.composite",
			wantVersion: "2.1.0",
			wantOrigin:  OriginLocal,
			wantDeps: map[string]string{
				"example.base":     "",
				"example.database": "1.4.0",
				"example.cache":    "0.9",
				"https://example.com/roles/legacy.tar.gz": "",
				"community.general":                       "",
			},
		},
		{
			name:        "meta/main.yaml is accepted as well as meta/main.yml",
			dir:         "role-yaml-ext",
			wantFQN:     "role-yaml-ext",
			wantVersion: "",
			wantOrigin:  OriginLocal,
			wantDeps:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRole(filepath.Join("testdata", tt.dir))
			if err != nil {
				t.Fatalf("ParseRole: %v", err)
			}
			if got.FQN() != tt.wantFQN {
				t.Errorf("FQN = %q, want %q", got.FQN(), tt.wantFQN)
			}
			if got.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", got.Version, tt.wantVersion)
			}
			if got.Origin != tt.wantOrigin {
				t.Errorf("Origin = %q, want %q", got.Origin, tt.wantOrigin)
			}
			if got.Kind != KindRole {
				t.Errorf("Kind = %q, want %q", got.Kind, KindRole)
			}
			if len(got.Dependencies) != len(tt.wantDeps) {
				t.Errorf("Dependencies = %v, want %v", got.Dependencies, tt.wantDeps)
			}
			for k, v := range tt.wantDeps {
				if got.Dependencies[k] != v {
					t.Errorf("Dependencies[%q] = %q, want %q", k, got.Dependencies[k], v)
				}
			}
		})
	}
}

// Roles carry no checksums anywhere. Emitting them at any other tier would let a consumer read
// "no hashes recorded" as a limitation of this tool rather than of the ecosystem (ADR-0005).
func TestParseRoleIsAlwaysNameVersionOnly(t *testing.T) {
	for _, dir := range []string{"role-galaxy", "role-local", "role-with-deps", "role-yaml-ext"} {
		got, err := ParseRole(filepath.Join("testdata", dir))
		if err != nil {
			t.Fatalf("%s: ParseRole: %v", dir, err)
		}
		if got.Tier != TierNameVersionOnly {
			t.Errorf("%s: Tier = %q, want %q", dir, got.Tier, TierNameVersionOnly)
		}
		if len(got.Files) != 0 {
			t.Errorf("%s: Files = %d, want 0 — roles have no file manifest", dir, len(got.Files))
		}
	}
}

func TestParseRoleNotARole(t *testing.T) {
	_, err := ParseRole(filepath.Join("testdata", "not-content"))
	if !errors.Is(err, ErrNotRole) {
		t.Fatalf("error = %v, want %v", err, ErrNotRole)
	}
}

func TestNormaliseVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v0.3.2", "0.3.2"},
		{"V1.0.0", "1.0.0"},
		{"3.5.0", "3.5.0"},
		{"", ""},
		{"v", "v"},
		// Not a version prefix: a name that merely starts with v.
		{"vault-role", "vault-role"},
		{"victoria", "victoria"},
	}
	for _, tt := range tests {
		if got := NormaliseVersion(tt.in); got != tt.want {
			t.Errorf("NormaliseVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Galaxy-installed roles are named "namespace.rolename" on disk. The fixtures are named for the
// case they exercise rather than for a namespace, so identity splits on the first dot.
func TestRoleIdentitySplitsOnDirectoryName(t *testing.T) {
	got, err := ParseRole(filepath.Join("testdata", "role-galaxy"))
	if err != nil {
		t.Fatalf("ParseRole: %v", err)
	}
	if got.Namespace != "" {
		t.Errorf("Namespace = %q, want empty for a directory name with no dot", got.Namespace)
	}
	if got.Name != "role-galaxy" {
		t.Errorf("Name = %q, want %q", got.Name, "role-galaxy")
	}
}
