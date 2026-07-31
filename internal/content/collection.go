package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// manifestFormat is the only MANIFEST.json/FILES.json format this parser understands.
//
// There is no JSON Schema for either file — the structure exists only as a dict literal in
// ansible-core's _build_manifest(). This constant is therefore a hard gate rather than a hint:
// on an unrecognised format we decline to parse, because mis-reading a checksum manifest is
// worse than not reading it. See ADR-0007.
const manifestFormat = 1

const (
	manifestFile = "MANIFEST.json"
	filesFile    = "FILES.json"
)

// ErrNotCollection is returned when a directory has no MANIFEST.json.
var ErrNotCollection = errors.New("no MANIFEST.json")

type manifest struct {
	CollectionInfo struct {
		Namespace    string            `json:"namespace"`
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		License      []string          `json:"license"`
		Dependencies map[string]string `json:"dependencies"`
		// Deliberately not parsed for identity: repository, documentation, homepage, issues,
		// authors, description. Real collections ship unedited skeleton placeholders in them.
	} `json:"collection_info"`
	FileManifestFile struct {
		Name   string `json:"name"`
		SHA256 string `json:"chksum_sha256"`
	} `json:"file_manifest_file"`
	Format int `json:"format"`
}

type filesManifest struct {
	Files []struct {
		Name   string `json:"name"`
		FType  string `json:"ftype"`
		SHA256 string `json:"chksum_sha256"`
	} `json:"files"`
	Format int `json:"format"`
}

// ParseCollection reads an installed collection directory.
//
// The directory is the leaf of an ansible_collections/<namespace>/<name> path. Identity comes
// from MANIFEST.json rather than from the directory names, because a collection built from a git
// checkout can be installed under a path that does not match its declared identity.
func ParseCollection(dir string) (Component, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Component{}, ErrNotCollection
		}
		return Component{}, fmt.Errorf("reading %s: %w", manifestFile, err)
	}

	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Component{}, fmt.Errorf("parsing %s: %w", manifestFile, err)
	}
	if m.Format != manifestFormat {
		return Component{}, fmt.Errorf("unsupported %s format %d (this build understands %d only)",
			manifestFile, m.Format, manifestFormat)
	}
	if m.CollectionInfo.Namespace == "" || m.CollectionInfo.Name == "" {
		return Component{}, fmt.Errorf("%s declares no namespace/name", manifestFile)
	}

	c := Component{
		Kind:           KindCollection,
		Namespace:      m.CollectionInfo.Namespace,
		Name:           m.CollectionInfo.Name,
		Version:        m.CollectionInfo.Version,
		Path:           dir,
		Origin:         OriginUnknown,
		Tier:           TierChecksummed,
		Coverage:       CoverageNotCovered,
		Dependencies:   m.CollectionInfo.Dependencies,
		Licenses:       m.CollectionInfo.License,
		FilesDigest:    m.FileManifestFile.SHA256,
		ManifestFormat: m.Format,
	}
	if c.Dependencies == nil {
		c.Dependencies = map[string]string{}
	}

	files, err := parseFiles(dir)
	if err != nil {
		return Component{}, err
	}
	c.Files = files

	return c, nil
}

func parseFiles(dir string) ([]File, error) {
	raw, err := os.ReadFile(filepath.Join(dir, filesFile))
	if err != nil {
		if os.IsNotExist(err) {
			// A MANIFEST without FILES is not a shape ansible-galaxy produces. Report it
			// rather than silently returning a collection with no integrity data, which
			// would be indistinguishable from a role.
			return nil, fmt.Errorf("%s present but %s missing", manifestFile, filesFile)
		}
		return nil, fmt.Errorf("reading %s: %w", filesFile, err)
	}

	var fm filesManifest
	if err := json.Unmarshal(raw, &fm); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filesFile, err)
	}
	if fm.Format != manifestFormat {
		return nil, fmt.Errorf("unsupported %s format %d (this build understands %d only)",
			filesFile, fm.Format, manifestFormat)
	}

	out := make([]File, 0, len(fm.Files))
	for _, e := range fm.Files {
		// Directory entries carry null checksums by design; they are not content.
		if e.FType != "file" {
			continue
		}
		out = append(out, File{Path: e.Name, SHA256: e.SHA256})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
