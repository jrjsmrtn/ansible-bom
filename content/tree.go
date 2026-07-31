// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CollectionsDirName is the directory ansible-galaxy installs collections into.
const CollectionsDirName = "ansible_collections"

// RolesDirName is the conventional directory for installed roles.
const RolesDirName = "roles"

// ScanCollections inventories an ansible_collections directory.
//
// The layout is <root>/<namespace>/<name>/. Namespace directories also contain
// "<ns>.<name>-<version>.info" marker directories written by ansible-galaxy; those are siblings of
// the namespace directories rather than of the collections, and are not inventoried — the version
// they encode is already in MANIFEST.json, which is authoritative. They are read only as an origin
// signal, per installMarkers.
func ScanCollections(root string) (Inventory, error) {
	var inv Inventory

	entries, err := os.ReadDir(root)
	if err != nil {
		return inv, fmt.Errorf("reading %s: %w", root, err)
	}

	markers := installMarkers(entries)

	for _, ns := range entries {
		if !ns.IsDir() || filepath.Ext(ns.Name()) == ".info" {
			continue
		}
		nsPath := filepath.Join(root, ns.Name())
		names, err := os.ReadDir(nsPath)
		if err != nil {
			inv.Problems = append(inv.Problems, Problem{Path: nsPath, Reason: err.Error()})
			continue
		}
		for _, n := range names {
			if !n.IsDir() {
				continue
			}
			dir := filepath.Join(nsPath, n.Name())
			c, err := ParseCollection(dir)
			switch {
			case errors.Is(err, ErrNotCollection):
				// Not every directory under a namespace is a collection.
				continue
			case err != nil:
				inv.Problems = append(inv.Problems, Problem{Path: dir, Reason: err.Error()})
				continue
			}
			if markers[c.FQN()] {
				c.Origin = OriginGalaxy
			}
			inv.Components = append(inv.Components, c)
		}
	}

	inv.sort()
	return inv, nil
}

// installMarkers returns the set of collections that have an "<ns>.<name>-<version>.info" marker
// directory, which ansible-galaxy writes alongside namespace directories.
//
// Presence is positive evidence of an ansible-galaxy install. Absence is NOT evidence of a git
// source: on a real control node, three collections lacked a marker — two genuinely installed from
// git, and one declared as an ordinary Galaxy collection. So a missing marker leaves the origin
// unknown rather than making it git. Overstating provenance would be worse than admitting
// ignorance.
func installMarkers(entries []os.DirEntry) map[string]bool {
	markers := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() || filepath.Ext(e.Name()) != ".info" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".info")
		// "<ns>.<name>-<version>" — the version is whatever follows the last hyphen.
		if i := strings.LastIndex(base, "-"); i > 0 {
			markers[base[:i]] = true
		}
	}
	return markers
}

// ScanRoles inventories a roles directory. The layout is <root>/<role>/, where a role is any
// directory containing meta/main.yml.
func ScanRoles(root string) (Inventory, error) {
	var inv Inventory

	entries, err := os.ReadDir(root)
	if err != nil {
		return inv, fmt.Errorf("reading %s: %w", root, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		r, err := ParseRole(dir)
		switch {
		case errors.Is(err, ErrNotRole):
			continue
		case err != nil:
			inv.Problems = append(inv.Problems, Problem{Path: dir, Reason: err.Error()})
			continue
		}
		inv.Components = append(inv.Components, r)
	}

	inv.sort()
	return inv, nil
}

// Scan inventories a content root containing any of ansible_collections/ and roles/.
//
// A missing subdirectory is not an error: control nodes legitimately have collections without
// roles, or the reverse. A root with neither yields an empty inventory rather than a failure,
// because "nothing installed here" is a valid answer that the caller should report as such.
func Scan(root string) (Inventory, error) {
	var inv Inventory

	// A root that does not exist is a mistake, not an empty answer. Silently returning nothing
	// would make a typo'd path indistinguishable from a control node with no content — and the
	// caller would get a valid, empty, entirely misleading BOM.
	fi, err := os.Stat(root)
	if err != nil {
		return inv, fmt.Errorf("content root %s: %w", root, err)
	}
	if !fi.IsDir() {
		return inv, fmt.Errorf("content root %s is not a directory", root)
	}

	for _, part := range []struct {
		dir string
		fn  func(string) (Inventory, error)
	}{
		{filepath.Join(root, CollectionsDirName), ScanCollections},
		{filepath.Join(root, RolesDirName), ScanRoles},
	} {
		if fi, err := os.Stat(part.dir); err != nil || !fi.IsDir() {
			continue
		}
		sub, err := part.fn(part.dir)
		if err != nil {
			return inv, err
		}
		inv.Components = append(inv.Components, sub.Components...)
		inv.Problems = append(inv.Problems, sub.Problems...)
	}

	inv.sort()
	return inv, nil
}

// sort orders components deterministically: collections before roles, then by name. Output that
// changes order between runs is useless for diffing, which is most of what this tool is for.
func (inv *Inventory) sort() {
	sort.SliceStable(inv.Components, func(i, j int) bool {
		a, b := inv.Components[i], inv.Components[j]
		if a.Kind != b.Kind {
			return a.Kind == KindCollection
		}
		return a.FQN() < b.FQN()
	})
	sort.SliceStable(inv.Problems, func(i, j int) bool {
		return inv.Problems[i].Path < inv.Problems[j].Path
	})
}
