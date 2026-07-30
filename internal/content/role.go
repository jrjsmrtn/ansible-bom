package content

import (
	"errors"
	"fmt"
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

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return Component{}, fmt.Errorf("reading %s: %w", metaPath, err)
	}

	var m roleMeta
	// Role metadata in the wild contains keys no schema anticipates. Unknown keys are ignored
	// rather than rejected: this tool inventories, it does not lint (ADR-0007).
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Component{}, fmt.Errorf("parsing %s: %w", metaPath, err)
	}

	namespace, name := roleIdentity(dir, m)

	c := Component{
		Kind:           KindRole,
		Namespace:      namespace,
		Name:           name,
		Path:           dir,
		Origin:         OriginLocal,
		Tier:           TierNameVersionOnly,
		Coverage:       CoverageNotCovered,
		Dependencies:   roleDependencies(m),
		ManifestFormat: 0, // roles have no manifest format
	}

	if v, ok := readInstallInfo(dir); ok {
		c.Version = NormaliseVersion(v)
		c.Origin = OriginGalaxy
	} else if m.GalaxyInfo.Version != "" {
		// A version declared by the author, not recorded by an installer. Weaker, but better
		// than nothing — and still not defaulted when absent.
		c.Version = NormaliseVersion(m.GalaxyInfo.Version)
	}

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

// roleIdentity derives namespace and name.
//
// Galaxy-installed roles land in a directory named "namespace.rolename"; that directory name is
// the only reliable source, because role metadata frequently omits namespace and role_name. When
// the directory carries no dot the role is local and has no namespace.
func roleIdentity(dir string, m roleMeta) (namespace, name string) {
	base := filepath.Base(dir)
	if ns, n, found := strings.Cut(base, "."); found {
		return ns, n
	}
	if m.GalaxyInfo.Namespace != "" && m.GalaxyInfo.RoleName != "" {
		return m.GalaxyInfo.Namespace, m.GalaxyInfo.RoleName
	}
	return "", base
}

func readInstallInfo(dir string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, roleMetaDir, installInfoFile))
	if err != nil {
		return "", false
	}
	var info installInfo
	if err := yaml.Unmarshal(raw, &info); err != nil {
		return "", false
	}
	if info.Version == "" {
		return "", false
	}
	return info.Version, true
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
