package content

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// CollectionsDirName is the directory ansible-galaxy installs collections into.
const CollectionsDirName = "ansible_collections"

// RolesDirName is the conventional directory for installed roles.
const RolesDirName = "roles"

// ScanCollections inventories an ansible_collections directory.
//
// The layout is <root>/<namespace>/<name>/. Namespace directories also contain
// "<ns>.<name>-<version>.info" marker directories written by ansible-galaxy; those are siblings of
// the namespace directories rather than of the collections, and are ignored — the version they
// encode is already in MANIFEST.json, which is authoritative.
func ScanCollections(root string) (Inventory, error) {
	var inv Inventory

	entries, err := os.ReadDir(root)
	if err != nil {
		return inv, fmt.Errorf("reading %s: %w", root, err)
	}

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
			inv.Components = append(inv.Components, c)
		}
	}

	inv.sort()
	return inv, nil
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
