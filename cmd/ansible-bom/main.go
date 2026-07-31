// Command ansible-bom inventories installed Ansible content and reports on it.
//
// See https://github.com/jrjsmrtn/ansible-bom. Not affiliated with Red Hat, Inc.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jrjsmrtn/ansible-bom/content"
	"github.com/jrjsmrtn/ansible-bom/internal/cyclonedx"
	"github.com/jrjsmrtn/ansible-bom/internal/drift"
	"github.com/jrjsmrtn/ansible-bom/internal/lockfile"
	"github.com/jrjsmrtn/ansible-bom/internal/requirements"
	"github.com/jrjsmrtn/ansible-bom/internal/verify"
)

// version is the tool's own version, injected at build time from the git tag:
//
//	go build -ldflags "-X main.version=$(git describe --tags)"
//
// It defaults to "dev" so an untagged local build never claims to be a release. Identifiers the
// tool emits are provisional until the `ansible` purl type is approved and implemented upstream,
// which is what gates 1.0 (ADR-0004).
var version = "dev"

const usage = `ansible-bom %s — inventory installed Ansible content.

Usage:
  ansible-bom lock  [flags] <root>...
  ansible-bom drift [flags] <root>...
  ansible-bom scan  [flags] <root>...
  ansible-bom verify [flags] <root>...

Commands:
  lock      Write a lockfile recording exactly what is installed.
  drift     Compare what is installed against what requirements.yml asks for.
  scan      Emit a CycloneDX bill of materials. Inventory only — see below.
  verify    Check installed files against the checksums recorded at install time.

Flags for 'lock':
  -o, --output <path>     write to a file instead of stdout
      --requirements      emit an installable requirements.yml projection instead
      --fail-on-problems  exit non-zero if any content could not be parsed

Flags for 'drift':
  -r, --requirements <path>  requirements.yml to compare against (required)
      --fail-on-drift        exit non-zero if reproducibility is compromised

Flags for 'scan':
  -o, --output <path>     write to a file instead of stdout
      --fail-on-problems  exit non-zero if any content could not be parsed

Flags for 'verify':
  -q, --quiet     report only failures
      --exit-zero always exit 0, even on a failed verification

'verify' exits non-zero on failure by default, unlike the other commands: that is what a
verification tool is for. It answers "has anything changed since installation?" — not "is this
what upstream published?", which needs the Galaxy server or a signature.

A <root> is a directory containing ansible_collections/ and/or roles/. Pass more than one when
they live apart, which is common — ansible.cfg is what decides where they are.

This tool inventories. It does not scan for vulnerabilities: no vulnerability database indexes
Ansible collections or roles, so an empty result from one means nothing.
`

// app holds the output streams, so the whole CLI is exercisable from tests without touching
// process state. Nothing below writes to os.Stdout or os.Stderr directly.
type app struct {
	stdout io.Writer
	stderr io.Writer
}

// usageError is returned for a malformed invocation, which exits 2 rather than 1 — the
// conventional distinction between "you asked wrongly" and "the thing you asked for failed".
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return usageError{fmt.Errorf(format, args...)}
}

// exitCode maps an error to a process exit status.
func exitCode(err error) int {
	var ue usageError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ue):
		return 2
	default:
		return 1
	}
}

func main() {
	a := &app{stdout: os.Stdout, stderr: os.Stderr}
	if err := a.run(os.Args[1:]); err != nil {
		fmt.Fprintf(a.stderr, "ansible-bom: %v\n", err)
		os.Exit(exitCode(err))
	}
}

func (a *app) run(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(a.stderr, usage, version)
		return usagef("no command given")
	}

	switch args[0] {
	case "lock":
		return a.runLock(args[1:])
	case "drift":
		return a.runDrift(args[1:])
	case "scan":
		return a.runScan(args[1:])
	case "verify":
		return a.runVerify(args[1:])
	case "version", "--version", "-v":
		fmt.Fprintf(a.stdout, "ansible-bom %s\n", version)
		return nil
	case "help", "--help", "-h":
		fmt.Fprintf(a.stdout, usage, version)
		return nil
	default:
		return usagef("unknown command %q — try 'ansible-bom help'", args[0])
	}
}

// scanRoots inventories every root given, accumulating both components and problems.
func (a *app) scanRoots(roots []string) (content.Inventory, error) {
	var inv content.Inventory
	for _, root := range roots {
		sub, err := content.Scan(root)
		if err != nil {
			return inv, err
		}
		inv.Components = append(inv.Components, sub.Components...)
		inv.Problems = append(inv.Problems, sub.Problems...)
	}
	return inv, nil
}

func (a *app) runLock(args []string) error {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var output string
	var asRequirements, failOnProblems bool
	fs.StringVar(&output, "output", "", "write to a file instead of stdout")
	fs.StringVar(&output, "o", "", "write to a file instead of stdout")
	fs.BoolVar(&asRequirements, "requirements", false, "emit an installable requirements.yml projection")
	fs.BoolVar(&failOnProblems, "fail-on-problems", false, "exit non-zero if any content could not be parsed")

	if err := fs.Parse(args); err != nil {
		return err
	}
	roots := fs.Args()
	if len(roots) == 0 {
		return usagef("no content root given — try 'ansible-bom help'")
	}

	inv, err := a.scanRoots(roots)
	if err != nil {
		return err
	}

	lock := lockfile.New(inv, "ansible-bom "+version, roots)

	var out []byte
	var omitted int
	if asRequirements {
		out, omitted, err = lockfile.Requirements(lock)
	} else {
		out, err = lockfile.Marshal(lock)
	}
	if err != nil {
		return err
	}

	if output == "" {
		if _, err := a.stdout.Write(out); err != nil {
			return err
		}
	} else if err := os.WriteFile(output, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", output, err)
	}

	report(a.stderr, lock, inv, omitted, asRequirements)

	if failOnProblems && len(inv.Problems) > 0 {
		return fmt.Errorf("%d component(s) could not be parsed", len(inv.Problems))
	}
	return nil
}

// report writes the human-readable summary to stderr, so it never contaminates piped output.
//
// Counts are deliberately never presented as a single total: a reader who sees "22 components"
// will take it for "22 verified", and only some of them carry any integrity data at all.
func report(w io.Writer, l lockfile.Lock, inv content.Inventory, omitted int, asRequirements bool) {
	s := l.Summary
	fmt.Fprintf(w, "%d collection(s), %d role(s)\n", s.Collections, s.Roles)
	fmt.Fprintf(w, "  %d pinned, %d with a content digest\n", s.Pinned, s.Checksummed)

	if s.Roles > 0 && s.Checksummed < s.Pinned {
		fmt.Fprintf(w, "  roles carry no checksums — this is an ecosystem gap, not a tool limitation\n")
	}

	if s.Unpinnable > 0 {
		fmt.Fprintf(w, "  %d NOT pinned (no version recorded on disk):\n", s.Unpinnable)
		for _, u := range l.Unpinnable {
			fmt.Fprintf(w, "    %s (%s)\n", u.Name, u.Kind)
		}
		if asRequirements && omitted > 0 {
			fmt.Fprintf(w, "  a tree rebuilt from this projection will be missing them\n")
		}
	}

	if len(inv.Problems) > 0 {
		fmt.Fprintf(w, "  %d NOT inventoried:\n", len(inv.Problems))
		for _, p := range inv.Problems {
			fmt.Fprintf(w, "    %s — %s\n", p.Path, strings.TrimSpace(p.Reason))
		}
	}

	fmt.Fprintf(w, "no vulnerability data: no database indexes Ansible content\n")
}

func (a *app) runDrift(args []string) error {
	fs := flag.NewFlagSet("drift", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var reqPath string
	var failOnDrift bool
	fs.StringVar(&reqPath, "requirements", "", "requirements.yml to compare against")
	fs.StringVar(&reqPath, "r", "", "requirements.yml to compare against")
	fs.BoolVar(&failOnDrift, "fail-on-drift", false, "exit non-zero if reproducibility is compromised")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if reqPath == "" {
		return usagef("--requirements is required: drift compares intent against reality, and needs both")
	}
	roots := fs.Args()
	if len(roots) == 0 {
		return usagef("no content root given — try 'ansible-bom help'")
	}

	req, err := requirements.Parse(reqPath)
	if err != nil {
		return err
	}

	inv, err := a.scanRoots(roots)
	if err != nil {
		return err
	}

	rep := drift.Compare(inv, req)
	reportDrift(a.stdout, rep, reqPath)

	if failOnDrift && !rep.Reproducible() {
		return fmt.Errorf("this control node cannot be reproduced from %s", reqPath)
	}
	return nil
}

// driftHeadings gives each finding kind a human heading, ordered worst-first to match the report.
var driftHeadings = []struct {
	kind    drift.Kind
	heading string
}{
	{drift.KindMutableSource, "Tracking a moving target"},
	{drift.KindUnpinnable, "Cannot be pinned"},
	{drift.KindVersionMismatch, "Installed version is not the declared one"},
	{drift.KindUndeclared, "Installed but never declared"},
	{drift.KindMissing, "Declared but not installed"},
	{drift.KindUnpinned, "Declared without an exact version"},
	{drift.KindFirstParty, "First-party content (not drift)"},
}

func reportDrift(w io.Writer, rep drift.Report, reqPath string) {
	fmt.Fprintf(w, "%d declared in %s, %d installed\n\n", rep.Declared, reqPath, rep.Installed)

	if len(rep.Findings) == 0 {
		fmt.Fprintf(w, "No drift: every declaration is pinned and every installed component was declared.\n")
		return
	}

	byKind := map[drift.Kind][]drift.Finding{}
	for _, f := range rep.Findings {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}

	for _, h := range driftHeadings {
		fs := byKind[h.kind]
		if len(fs) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s (%d)\n", h.heading, len(fs))
		fmt.Fprintf(w, "  %s\n", fs[0].Detail)
		for _, f := range fs {
			switch {
			case f.Declared != "" && f.Installed != "":
				fmt.Fprintf(w, "    %-40s declared %s, installed %s\n", f.Component, f.Declared, f.Installed)
			case f.Declared != "":
				fmt.Fprintf(w, "    %-40s declared %s\n", f.Component, f.Declared)
			case f.Installed != "":
				fmt.Fprintf(w, "    %-40s installed %s\n", f.Component, f.Installed)
			default:
				fmt.Fprintf(w, "    %s\n", f.Component)
			}
		}
		fmt.Fprintln(w)
	}

	if rep.Reproducible() {
		fmt.Fprintf(w, "Reproducible: yes — nothing installed is unpinnable or mismatched.\n")
	} else {
		fmt.Fprintf(w, "Reproducible: NO — this node cannot be rebuilt as it stands.\n")
	}
}

// runScan emits a CycloneDX BOM.
//
// The subcommand is named for what SBOM tooling calls this operation, which invites the wrong
// reading: it inventories, it does not assess. ADR-0006 requires that never be implied, so the
// summary says so explicitly every time.
func (a *app) runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var output string
	var failOnProblems bool
	fs.StringVar(&output, "output", "", "write to a file instead of stdout")
	fs.StringVar(&output, "o", "", "write to a file instead of stdout")
	fs.BoolVar(&failOnProblems, "fail-on-problems", false, "exit non-zero if any content could not be parsed")

	if err := fs.Parse(args); err != nil {
		return err
	}
	roots := fs.Args()
	if len(roots) == 0 {
		return usagef("no content root given — try 'ansible-bom help'")
	}

	inv, err := a.scanRoots(roots)
	if err != nil {
		return err
	}

	bom, err := cyclonedx.New(inv, cyclonedx.Options{
		ToolName: "ansible-bom", ToolVersion: version, Roots: roots,
	})
	if err != nil {
		return err
	}
	out, err := cyclonedx.Marshal(bom)
	if err != nil {
		return err
	}

	if output == "" {
		if _, err := a.stdout.Write(out); err != nil {
			return err
		}
	} else if err := os.WriteFile(output, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", output, err)
	}

	collections, roles, checksummed, unversioned := inv.Counts()
	fmt.Fprintf(a.stderr, "CycloneDX %s: %d component(s) — %d collection(s), %d role(s)\n",
		cyclonedx.SpecVersion, len(inv.Components), collections, roles)
	fmt.Fprintf(a.stderr, "  %d with a content digest, %d without a recorded version\n",
		checksummed, unversioned)
	if len(inv.Problems) > 0 {
		fmt.Fprintf(a.stderr, "  %d NOT inventoried — the BOM declares itself incomplete:\n", len(inv.Problems))
		for _, p := range inv.Problems {
			fmt.Fprintf(a.stderr, "    %s — %s\n", p.Path, strings.TrimSpace(p.Reason))
		}
	}
	fmt.Fprintf(a.stderr, "identifiers are PROVISIONAL: the `ansible` purl type is not yet registered\n")
	fmt.Fprintf(a.stderr, "this is an inventory, not a vulnerability assessment: no database indexes Ansible content\n")

	if failOnProblems && len(inv.Problems) > 0 {
		return fmt.Errorf("%d component(s) could not be parsed", len(inv.Problems))
	}
	return nil
}

func (a *app) runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	var quiet, exitZero bool
	fs.BoolVar(&quiet, "quiet", false, "report only failures")
	fs.BoolVar(&quiet, "q", false, "report only failures")
	fs.BoolVar(&exitZero, "exit-zero", false, "always exit 0, even on a failed verification")

	if err := fs.Parse(args); err != nil {
		return err
	}
	roots := fs.Args()
	if len(roots) == 0 {
		return usagef("no content root given — try 'ansible-bom help'")
	}

	inv, err := a.scanRoots(roots)
	if err != nil {
		return err
	}

	rep := verify.Inventory(inv)
	reportVerify(a.stdout, rep, inv, quiet)

	if !rep.OK() && !exitZero {
		return fmt.Errorf("verification failed")
	}
	return nil
}

func reportVerify(w io.Writer, rep verify.Report, inv content.Inventory, quiet bool) {
	verified, modified, unverifiable, errored := rep.Counts()

	for _, res := range rep.Results {
		switch res.Status {
		case verify.StatusVerified:
			if !quiet {
				fmt.Fprintf(w, "  ok           %-40s %d file(s)\n", res.Component.FQN(), res.Checked)
			}
		case verify.StatusUnverifiable:
			if !quiet {
				fmt.Fprintf(w, "  unverifiable %-40s %s\n", res.Component.FQN(), res.Note)
			}
		case verify.StatusError:
			fmt.Fprintf(w, "  ERROR        %-40s %s\n", res.Component.FQN(), res.Note)
		case verify.StatusModified:
			fmt.Fprintf(w, "  FAILED       %-40s %d of %d file(s) differ\n",
				res.Component.FQN(), len(res.Problems), res.Checked)
			if res.Note != "" {
				fmt.Fprintf(w, "                 %s\n", res.Note)
			}
			for i, p := range res.Problems {
				if i == 10 {
					fmt.Fprintf(w, "                 ... and %d more\n", len(res.Problems)-10)
					break
				}
				fmt.Fprintf(w, "                 %-9s %s\n", p.Problem, p.Path)
			}
		}
	}

	fmt.Fprintf(w, "\n%d verified, %d failed, %d unverifiable", verified, modified, unverifiable)
	if errored > 0 {
		fmt.Fprintf(w, ", %d errored", errored)
	}
	fmt.Fprintln(w)

	if unverifiable > 0 {
		fmt.Fprintf(w, "unverifiable is not a pass: no checksums exist for Ansible roles\n")
	}
	if len(inv.Problems) > 0 {
		fmt.Fprintf(w, "%d component(s) could not be inventoried and were therefore not checked\n", len(inv.Problems))
	}
	fmt.Fprintf(w, "this checks against what was recorded at install time, not against what upstream published\n")
}
