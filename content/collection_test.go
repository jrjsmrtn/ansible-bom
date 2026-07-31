// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCollection(t *testing.T) {
	tests := []struct {
		name         string
		dir          string
		wantFQN      string
		wantVersion  string
		wantDeps     map[string]string
		wantMinFiles int
	}{
		{
			name:         "galaxy installed collection",
			dir:          "collection-galaxy",
			wantFQN:      "community.general",
			wantVersion:  "11.4.0",
			wantDeps:     map[string]string{},
			wantMinFiles: 6,
		},
		{
			name:        "collection declaring a range constraint",
			dir:         "collection-with-deps",
			wantFQN:     "community.windows",
			wantVersion: "3.0.1",
			// Recorded verbatim, never resolved (ADR-0003).
			wantDeps:     map[string]string{"ansible.windows": ">=3.0.0,<4.0.0"},
			wantMinFiles: 6,
		},
		{
			name:         "identity survives an unedited skeleton placeholder elsewhere in the manifest",
			dir:          "collection-placeholder-repo",
			wantFQN:      "example.widget",
			wantVersion:  "0.1.0",
			wantDeps:     map[string]string{},
			wantMinFiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCollection(filepath.Join("testdata", tt.dir))
			if err != nil {
				t.Fatalf("ParseCollection: %v", err)
			}
			if got.FQN() != tt.wantFQN {
				t.Errorf("FQN = %q, want %q", got.FQN(), tt.wantFQN)
			}
			if got.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", got.Version, tt.wantVersion)
			}
			if got.Kind != KindCollection {
				t.Errorf("Kind = %q, want %q", got.Kind, KindCollection)
			}
			if got.Tier != TierChecksummed {
				t.Errorf("Tier = %q, want %q — collections carry per-file checksums", got.Tier, TierChecksummed)
			}
			if got.Coverage != CoverageNotCovered {
				t.Errorf("Coverage = %q, want %q (ADR-0006)", got.Coverage, CoverageNotCovered)
			}
			if len(got.Dependencies) != len(tt.wantDeps) {
				t.Errorf("Dependencies = %v, want %v", got.Dependencies, tt.wantDeps)
			}
			for k, v := range tt.wantDeps {
				if got.Dependencies[k] != v {
					t.Errorf("Dependencies[%q] = %q, want %q", k, got.Dependencies[k], v)
				}
			}
			if len(got.Files) < tt.wantMinFiles {
				t.Errorf("Files = %d, want >= %d", len(got.Files), tt.wantMinFiles)
			}
		})
	}
}

// Directory entries in FILES.json carry null checksums by design. Admitting them would
// manufacture components with empty hashes.
func TestParseCollectionSkipsDirectoryEntries(t *testing.T) {
	got, err := ParseCollection(filepath.Join("testdata", "collection-galaxy"))
	if err != nil {
		t.Fatalf("ParseCollection: %v", err)
	}
	for _, f := range got.Files {
		if f.SHA256 == "" {
			t.Errorf("file %q has no sha256 — a directory entry leaked into Files", f.Path)
		}
		if f.Path == "." {
			t.Errorf("directory entry %q leaked into Files", f.Path)
		}
	}
}

func TestParseCollectionFilesAreSorted(t *testing.T) {
	got, err := ParseCollection(filepath.Join("testdata", "collection-galaxy"))
	if err != nil {
		t.Fatalf("ParseCollection: %v", err)
	}
	for i := 1; i < len(got.Files); i++ {
		if got.Files[i-1].Path > got.Files[i].Path {
			t.Fatalf("Files not sorted: %q before %q", got.Files[i-1].Path, got.Files[i].Path)
		}
	}
}

func TestParseCollectionErrors(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		wantErrIs   error
		wantErrText string
	}{
		{
			name:      "directory that is not a collection",
			dir:       "not-content",
			wantErrIs: ErrNotCollection,
		},
		{
			// The ADR-0007 gate: decline rather than guess. Mis-reading a checksum manifest is
			// worse than not reading it.
			name:        "unrecognised manifest format",
			dir:         "collection-future-format",
			wantErrText: "unsupported MANIFEST.json format 2",
		},
		{
			// Must not silently degrade to a checksum-less collection, which would be
			// indistinguishable from a role.
			name:        "manifest without a files manifest",
			dir:         "collection-no-files",
			wantErrText: "FILES.json missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCollection(filepath.Join("testdata", tt.dir))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Fatalf("error = %v, want %v", err, tt.wantErrIs)
			}
			if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErrText)
			}
		})
	}
}
