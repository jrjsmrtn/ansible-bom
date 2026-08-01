// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package purl constructs Package URL identifiers for Ansible content.
//
// This is the ONLY place identifiers are built. There is no registered `ansible` purl type yet —
// purl-spec#854 proposes one and is still open — so every identifier this tool emits is
// provisional, and v1.0 is gated on that type being approved and implemented upstream. Keeping
// construction in one function makes that change one edit. See ADR-0004.
package purl

import (
	"strings"

	"github.com/jrjsmrtn/ansible-bom/content"
)

// Type is the proposed purl type from purl-spec#854.
const Type = "ansible"

// ProposalURL is the upstream proposal this package conforms to.
//
// Conformance is deliberate and tracked: the type is not registered yet, so the target is a moving
// one. A vendored snapshot of the proposal and conformance_test.go pin the agreement, and fail if
// either side moves. See ADR-0004.
const ProposalURL = "https://github.com/package-url/purl-spec/pull/854"

// Status describes the stability of identifiers this package emits. It is carried into every
// document so a consumer cannot mistake a provisional identifier for a settled one.
const Status = "provisional: the `ansible` purl type is proposed but not yet registered"

// RoleQualifier marks a purl as naming a legacy role rather than a collection.
//
// This is the one deliberate extension to the upstream proposal, which is scoped to collections
// ("Ansible Collection") and says nothing about roles. Galaxy namespaces both, so
// "author/name@version" is ambiguous between them and a role and a collection sharing a name would
// otherwise collide.
//
// Kept as an extension rather than dropped: the ambiguity is real and silently emitting colliding
// identifiers would be worse than carrying a qualifier the proposal has not yet considered. A
// separate proposal covering roles is intended. NOT yet reported upstream.
const RoleQualifier = "kind=role"

// For builds the identifier for a component.
//
// Deliberately omitted: the `vcs_url` qualifier. It is the actively contested part of the
// proposal — reviewers disagree over ansible's comma-before-version syntax — and git-sourced
// content is reported through the drift channel instead, where a mutable source is a finding
// rather than an identifier detail.
func For(c content.Component) string {
	var b strings.Builder
	b.WriteString("pkg:")
	b.WriteString(Type)
	b.WriteString("/")

	// Namespace is a separate, required component in the proposal — NOT part of the name. Ansible
	// writes the identity as "cisco.aci"; the purl is pkg:ansible/cisco/aci.
	//
	// It is emitted only when one was actually observed. The proposal marks it required because it
	// models Galaxy-installed collections, which always have one; content found on disk without
	// install metadata — a locally-authored role in roles/ — genuinely has none. Inventing a
	// namespace to satisfy the schema would fabricate identity, which is the one thing this tool
	// must never do. Such a purl is knowingly incomplete, and the component is reported as
	// local-origin so the gap is visible rather than implied.
	if c.Namespace != "" {
		b.WriteString(escape(normalise(c.Namespace)))
		b.WriteString("/")
	}
	b.WriteString(escape(normalise(c.Name)))

	if c.Version != "" {
		b.WriteString("@")
		b.WriteString(escape(c.Version))
	}
	if c.Kind == content.KindRole {
		b.WriteString("?")
		b.WriteString(RoleQualifier)
	}
	return b.String()
}

// normalise lowercases a namespace or name, which the proposal requires of both
// (case_sensitive: false, "Must be lowercased"). Galaxy already restricts collection namespaces
// and names to lowercase, so this bites only on locally-authored role directories. The component's
// own Name is left untouched — this normalisation applies to the identifier, not to what the tool
// reports having found on disk.
func normalise(s string) string { return strings.ToLower(s) }

// escape percent-encodes the characters purl reserves. Ansible names are conservative — a
// namespace and name are restricted to lowercase alphanumerics and underscores — but versions
// and locally-authored role names are not guaranteed to be.
func escape(s string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~.+"
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && strings.ContainsRune(safe, r) {
			b.WriteRune(r)
			continue
		}
		for _, by := range []byte(string(r)) {
			b.WriteByte('%')
			const hex = "0123456789ABCDEF"
			b.WriteByte(hex[by>>4])
			b.WriteByte(hex[by&0x0f])
		}
	}
	return b.String()
}
