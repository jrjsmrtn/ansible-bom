package purl

import (
	"strings"
	"testing"

	"github.com/jrjsmrtn/ansible-bom/internal/content"
)

func TestFor(t *testing.T) {
	tests := []struct {
		name string
		comp content.Component
		want string
	}{
		{
			name: "collection",
			comp: content.Component{Kind: content.KindCollection, Namespace: "community", Name: "general", Version: "11.4.0"},
			want: "pkg:ansible/community.general@11.4.0",
		},
		{
			// purl-spec#854 addresses collections and says nothing about roles, yet Galaxy
			// namespaces both — so a discriminator is needed or the two would collide.
			name: "role carries a kind qualifier",
			comp: content.Component{Kind: content.KindRole, Namespace: "jborean93", Name: "win_openssh", Version: "0.3.2"},
			want: "pkg:ansible/jborean93.win_openssh@0.3.2?kind=role",
		},
		{
			name: "unversioned content omits the version rather than inventing one",
			comp: content.Component{Kind: content.KindRole, Name: "site_common"},
			want: "pkg:ansible/site_common?kind=role",
		},
		{
			name: "collection with no namespace",
			comp: content.Component{Kind: content.KindCollection, Name: "orphan", Version: "1.0.0"},
			want: "pkg:ansible/orphan@1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := For(tt.comp); got != tt.want {
				t.Errorf("For() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The contested qualifier from the upstream proposal must never appear: reviewers disagree about
// its syntax, and git-sourced content is reported through drift instead (ADR-0004).
func TestForNeverEmitsVcsURL(t *testing.T) {
	got := For(content.Component{
		Kind: content.KindCollection, Namespace: "example", Name: "widget", Version: "0.1.0",
		Origin: content.OriginGit, Path: "/srv/x",
	})
	if strings.Contains(got, "vcs_url") {
		t.Errorf("For() = %q, must not carry vcs_url", got)
	}
}

func TestEscape(t *testing.T) {
	tests := []struct{ in, want string }{
		{"community.general", "community.general"},
		{"1.0.0-beta.1", "1.0.0-beta.1"},
		{"a b", "a%20b"},
		{"a/b", "a%2Fb"},
		{"café", "caf%C3%A9"},
	}
	for _, tt := range tests {
		if got := escape(tt.in); got != tt.want {
			t.Errorf("escape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// A role name containing a character purl reserves must not produce a malformed identifier.
func TestForEscapesAwkwardNames(t *testing.T) {
	got := For(content.Component{Kind: content.KindRole, Name: "my role"})
	if strings.Contains(got, " ") {
		t.Errorf("For() = %q, contains an unescaped space", got)
	}
}
