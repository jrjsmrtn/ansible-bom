// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	roleMetaDir     = "meta"
	installInfoFile = ".galaxy_install_info"
)

// roleMetaFiles are the accepted names for role metadata, in precedence order.
var roleMetaFiles = []string{"main.yml", "main.yaml", "main"}

// ErrNotRole is returned when a directory has no role metadata.
var ErrNotRole = errors.New("no meta/main.yml")

// RoleMeta is the identity and dependencies a role's meta/main.yml declares.
type RoleMeta struct {
	Namespace    string
	RoleName     string
	Version      string
	Dependencies map[string]string
}

// DecodeRoleMeta decodes meta/main.yml, covering both shapes the ansible-meta schema describes
// as v1/v2 (ADR-0007). Unknown keys are ignored: this tool inventories, it does not lint.
//
// Reader-based so the same logic serves the filesystem walk and a syft cataloger (ADR-0008).
func DecodeRoleMeta(r io.Reader) (RoleMeta, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return RoleMeta{}, fmt.Errorf("reading role metadata: %w", err)
	}
	var m roleMeta
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return RoleMeta{}, fmt.Errorf("parsing role metadata: %w", err)
	}
	return RoleMeta{
		Namespace:    m.GalaxyInfo.Namespace,
		RoleName:     m.GalaxyInfo.RoleName,
		Version:      m.GalaxyInfo.Version,
		Dependencies: roleDependencies(m),
	}, nil
}

// DecodeInstallInfo decodes meta/.galaxy_install_info, returning the version ansible-galaxy
// recorded at install time. install_date is deliberately not returned: it is written in the
// installing user's locale and must be treated as opaque (ADR-0002 §5).
func DecodeInstallInfo(r io.Reader) (string, bool) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return "", false
	}
	var info installInfo
	if err := yaml.Unmarshal(raw, &info); err != nil || info.Version == "" {
		return "", false
	}
	return info.Version, true
}

// ComponentFromRoleMeta builds a Component from decoded role metadata.
//
// name is the role's directory name, which is the only reliable source of identity — role
// metadata frequently omits namespace and role_name. installedVersion is what an installer
// recorded, if anything; when absent the author-declared version is used, and when that is absent
// too the component is left unversioned rather than given a placeholder.
func ComponentFromRoleMeta(name string, m RoleMeta, installedVersion string) Component {
	c := Component{
		Kind:         KindRole,
		Dependencies: m.Dependencies,
		Tier:         TierNameVersionOnly,
		Coverage:     CoverageNotCovered,
		Origin:       OriginLocal,
	}
	if ns, n, found := strings.Cut(name, "."); found {
		c.Namespace, c.Name = ns, n
	} else if m.Namespace != "" && m.RoleName != "" {
		c.Namespace, c.Name = m.Namespace, m.RoleName
	} else {
		c.Name = name
	}
	switch {
	case installedVersion != "":
		c.Version = NormaliseVersion(installedVersion)
		c.Origin = OriginGalaxy
	case m.Version != "":
		c.Version = NormaliseVersion(m.Version)
	}
	return c
}

// installInfo is written by ansible-galaxy when it installs a role.
//
// install_date is deliberately typed as a string and never parsed: ansible-galaxy writes it in
// the installing user's locale (values such as "Sam  3 sep 14:26:34 2022" and "Mer  1 déc ..."
// occur in real trees). It is not read at all here — recorded only as a note for future
// maintainers who might be tempted. See ADR-0002 §5.
type installInfo struct {
	Version string `yaml:"version"`
}

// roleMeta covers both shapes the ansible-meta schema describes as v1/v2 (ADR-0007).
//
// v1 nests galaxy metadata under galaxy_info; dependencies sit at the top level in both.
// Dependencies entries are polymorphic: a bare string, or a mapping with `role`/`name`/`src`.
type roleMeta struct {
	GalaxyInfo struct {
		Namespace string `yaml:"namespace"`
		RoleName  string `yaml:"role_name"`
		Author    string `yaml:"author"`
		Version   string `yaml:"version"`
		License   any    `yaml:"license"`
	} `yaml:"galaxy_info"`
	Dependencies []yaml.Node `yaml:"dependencies"`
	Collections  []string    `yaml:"collections"`
}

// ParseRole reads an installed role directory.
//
// Roles carry no checksums and no manifest — their entire recorded identity is a version string
// that ansible-galaxy writes into meta/.galaxy_install_info at install time, absent altogether
// for roles that were never installed from Galaxy. The result is always TierNameVersionOnly.
// See ADR-0005.
func ParseRole(dir string) (Component, error) {
	metaPath, ok := findRoleMeta(dir)
	if !ok {
		return Component{}, ErrNotRole
	}

	f, err := os.Open(metaPath)
	if err != nil {
		return Component{}, fmt.Errorf("reading %s: %w", metaPath, err)
	}
	defer f.Close()

	m, err := DecodeRoleMeta(f)
	if err != nil {
		return Component{}, fmt.Errorf("%s: %w", metaPath, err)
	}

	installed, _ := readInstallInfo(dir)

	c := ComponentFromRoleMeta(filepath.Base(dir), m, installed)
	c.Path = dir
	return c, nil
}

func findRoleMeta(dir string) (string, bool) {
	for _, name := range roleMetaFiles {
		p := filepath.Join(dir, roleMetaDir, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

func readInstallInfo(dir string) (string, bool) {
	f, err := os.Open(filepath.Join(dir, roleMetaDir, installInfoFile))
	if err != nil {
		return "", false
	}
	defer f.Close()
	return DecodeInstallInfo(f)
}

// roleDependencies flattens role meta dependencies, which may be bare strings or mappings.
// Constraints are recorded verbatim; nothing is resolved.
func roleDependencies(m roleMeta) map[string]string {
	deps := map[string]string{}
	for _, node := range m.Dependencies {
		switch node.Kind {
		case yaml.ScalarNode:
			if node.Value != "" {
				deps[node.Value] = ""
			}
		case yaml.MappingNode:
			var entry struct {
				Role    string `yaml:"role"`
				Name    string `yaml:"name"`
				Src     string `yaml:"src"`
				Version string `yaml:"version"`
			}
			if err := node.Decode(&entry); err != nil {
				continue
			}
			key := firstNonEmpty(entry.Role, entry.Name, entry.Src)
			if key != "" {
				deps[key] = entry.Version
			}
		}
	}
	for _, c := range m.Collections {
		deps[c] = ""
	}
	return deps
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// NormaliseVersion strips a leading "v" from role versions.
//
// Galaxy role versions are inconsistently prefixed — "v0.3.2" and "3.5.0" occur side by side in
// the same tree — because the version is whatever the author tagged. Collections do not have this
// problem; their versions are semver-validated at build time.
func NormaliseVersion(v string) string {
	if len(v) > 1 && (v[0] == 'v' || v[0] == 'V') {
		rest := v[1:]
		if rest[0] >= '0' && rest[0] <= '9' {
			return rest
		}
	}
	return v
}
