// Package verify checks installed files against the checksums recorded when they were installed.
//
// What this establishes, and what it does not, matters more than usual here.
//
// The integrity chain is MANIFEST.json → FILES.json → the files. MANIFEST.json records a sha256
// for FILES.json, and FILES.json records one for every file. Nothing records MANIFEST.json's own
// hash: it is the root of trust, and it cannot be verified locally at all. An adversary who edits
// a file, regenerates FILES.json and updates MANIFEST.json accordingly defeats this check
// completely.
//
// So this answers "has anything changed since installation?" — a question about local tampering
// and accidental edits. It does NOT answer "is this what upstream published?", which needs the
// Galaxy server (`ansible-galaxy collection verify`, which requires network) or a signature.
//
// Roles are not verifiable at any level: no checksums exist for them anywhere. They are reported
// as unverifiable and never as passing (ADR-0005).
package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/jrjsmrtn/ansible-bom/content"
)

// Status is the outcome for one component.
type Status string

const (
	// StatusVerified: every recorded file is present and matches.
	StatusVerified Status = "verified"
	// StatusModified: at least one file is missing, changed, or unreadable.
	StatusModified Status = "modified"
	// StatusUnverifiable: no checksums exist for this kind of content. Never a pass.
	StatusUnverifiable Status = "unverifiable"
	// StatusError: verification could not be carried out.
	StatusError Status = "error"
)

// FileProblem is one file that did not match.
type FileProblem struct {
	Path     string
	Problem  string // "missing", "modified", "unreadable"
	Expected string
	Actual   string
}

// Result is the outcome for one component.
type Result struct {
	Component content.Component
	Status    Status

	// ManifestIntact reports whether FILES.json matched the sha256 MANIFEST.json records for
	// it. When false, the per-file results below are not trustworthy — the list of expected
	// checksums has itself been altered.
	ManifestIntact bool

	Checked  int
	Problems []FileProblem
	Note     string
}

// Report is the outcome for an inventory.
type Report struct {
	Results []Result
}

// noteUnverifiable explains a role's status in terms of the ecosystem rather than the tool.
const noteUnverifiable = "no checksums exist for Ansible roles; nothing to verify against"

// Inventory verifies every component in an inventory.
func Inventory(inv content.Inventory) Report {
	var rep Report
	for _, c := range inv.Components {
		rep.Results = append(rep.Results, Component(c))
	}
	sort.Slice(rep.Results, func(i, j int) bool {
		return rep.Results[i].Component.FQN() < rep.Results[j].Component.FQN()
	})
	return rep
}

// Component verifies one component.
func Component(c content.Component) Result {
	if c.Tier != content.TierChecksummed {
		return Result{Component: c, Status: StatusUnverifiable, Note: noteUnverifiable}
	}

	res := Result{Component: c}

	// Check the file manifest before trusting anything it says.
	intact, err := manifestIntact(c)
	if err != nil {
		return Result{Component: c, Status: StatusError, Note: err.Error()}
	}
	res.ManifestIntact = intact
	if !intact {
		res.Note = "FILES.json does not match the checksum MANIFEST.json records for it: " +
			"the list of expected checksums has itself been altered, so per-file results below are unreliable"
	}

	for _, f := range c.Files {
		res.Checked++
		path := filepath.Join(c.Path, f.Path)

		actual, err := fileSHA256(path)
		switch {
		case os.IsNotExist(err):
			res.Problems = append(res.Problems, FileProblem{Path: f.Path, Problem: "missing", Expected: f.SHA256})
			continue
		case err != nil:
			res.Problems = append(res.Problems, FileProblem{Path: f.Path, Problem: "unreadable", Expected: f.SHA256})
			continue
		}
		if actual != f.SHA256 {
			res.Problems = append(res.Problems, FileProblem{
				Path: f.Path, Problem: "modified", Expected: f.SHA256, Actual: actual,
			})
		}
	}

	if len(res.Problems) == 0 && intact {
		res.Status = StatusVerified
	} else {
		res.Status = StatusModified
	}
	return res
}

// manifestIntact compares FILES.json against the digest MANIFEST.json records for it.
func manifestIntact(c content.Component) (bool, error) {
	if c.FilesDigest == "" {
		// A collection whose manifest records no digest for its file manifest. Not a shape
		// ansible-galaxy produces; report rather than assume either way.
		return false, fmt.Errorf("MANIFEST.json records no checksum for FILES.json")
	}
	actual, err := fileSHA256(filepath.Join(c.Path, "FILES.json"))
	if err != nil {
		return false, fmt.Errorf("reading FILES.json: %w", err)
	}
	return actual == c.FilesDigest, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Counts summarises a report. Verified and unverifiable are returned separately and must stay
// that way in any output: collapsing them would report a role as having passed a check that was
// never run (ADR-0005).
func (r Report) Counts() (verified, modified, unverifiable, errored int) {
	for _, res := range r.Results {
		switch res.Status {
		case StatusVerified:
			verified++
		case StatusModified:
			modified++
		case StatusUnverifiable:
			unverifiable++
		case StatusError:
			errored++
		}
	}
	return
}

// OK reports whether verification found nothing wrong.
//
// Unverifiable components do not make this false: nothing was checked, so nothing failed. They
// are reported separately, and a caller that treats "OK" as "everything is verified" is making an
// error this package cannot prevent — hence Counts.
func (r Report) OK() bool {
	for _, res := range r.Results {
		if res.Status == StatusModified || res.Status == StatusError {
			return false
		}
	}
	return true
}
