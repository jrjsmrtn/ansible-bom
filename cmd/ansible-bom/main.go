// Command ansible-bom inventories installed Ansible content and reports on it.
//
// See https://github.com/jrjsmrtn/ansible-bom. Not affiliated with Red Hat, Inc.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jrjsmrtn/ansible-bom/internal/content"
	"github.com/jrjsmrtn/ansible-bom/internal/lockfile"
)

// version is the tool's own version. Identifiers it emits are provisional until the `ansible`
// purl type is approved and implemented upstream, which is what gates 1.0 (ADR-0004).
const version = "0.1.0"

const usage = `ansible-bom %s — inventory installed Ansible content.

Usage:
  ansible-bom lock [flags] <root>...

Commands:
  lock      Write a lockfile recording exactly what is installed.

Flags for 'lock':
  -o, --output <path>     write to a file instead of stdout
      --requirements      emit an installable requirements.yml projection instead
      --fail-on-problems  exit non-zero if any content could not be parsed

A <root> is a directory containing ansible_collections/ and/or roles/. Pass more than one when
they live apart, which is common — ansible.cfg is what decides where they are.

This tool inventories. It does not scan for vulnerabilities: no vulnerability database indexes
Ansible collections or roles, so an empty result from one means nothing.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ansible-bom: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}

	switch args[0] {
	case "lock":
		return runLock(args[1:])
	case "version", "--version", "-v":
		fmt.Printf("ansible-bom %s\n", version)
		return nil
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
		return nil
	default:
		return fmt.Errorf("unknown command %q — try 'ansible-bom help'", args[0])
	}
}

func runLock(args []string) error {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

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
		return fmt.Errorf("no content root given — try 'ansible-bom help'")
	}

	var inv content.Inventory
	for _, root := range roots {
		sub, err := content.Scan(root)
		if err != nil {
			return err
		}
		inv.Components = append(inv.Components, sub.Components...)
		inv.Problems = append(inv.Problems, sub.Problems...)
	}

	lock := lockfile.New(inv, "ansible-bom "+version, roots)

	var out []byte
	var err error
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
		if _, err := os.Stdout.Write(out); err != nil {
			return err
		}
	} else if err := os.WriteFile(output, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", output, err)
	}

	report(os.Stderr, lock, inv, omitted, asRequirements)

	if failOnProblems && len(inv.Problems) > 0 {
		return fmt.Errorf("%d component(s) could not be parsed", len(inv.Problems))
	}
	return nil
}

// report writes the human-readable summary to stderr, so it never contaminates piped output.
//
// Counts are deliberately never presented as a single total: a reader who sees "22 components"
// will take it for "22 verified", and only some of them carry any integrity data at all.
func report(w *os.File, l lockfile.Lock, inv content.Inventory, omitted int, asRequirements bool) {
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
