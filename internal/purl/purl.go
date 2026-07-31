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

// ProposalURL is the upstream proposal this implementation follows.
const ProposalURL = "https://github.com/package-url/purl-spec/pull/854"

// Status describes the stability of identifiers this package emits. It is carried into every
// document so a consumer cannot mistake a provisional identifier for a settled one.
const Status = "provisional: the `ansible` purl type is proposed but not yet registered"

// RoleQualifier marks a purl as naming a legacy role rather than a collection.
//
// This is an extension to the upstream proposal, not part of it: purl-spec#854 addresses
// collections and says nothing about roles, yet "author.name@version" is ambiguous between the
// two — Galaxy namespaces both. Without a discriminator, a role and a collection sharing a name
// would collide. Reported upstream; revisit when the type is registered.
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
	b.WriteString(escape(c.FQN()))

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
