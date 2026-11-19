package acl

import "testing"

func TestMatchPathSegmentBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		target  string
		want    bool
	}{
		// INC-2024-04-02: acme-evil must not be acme.
		{"evil sibling", "/srv/tenants/acme/**", "/srv/tenants/acme-evil/x", false},
		{"prefix sibling", "/srv/tenants/acme/**", "/srv/tenants/acmex", false},
		{"exact dir", "/srv/tenants/acme/**", "/srv/tenants/acme", true},
		{"below dir", "/srv/tenants/acme/**", "/srv/tenants/acme/reports/q1.csv", true},
		// "*" matches exactly one segment and never crosses a slash.
		{"star one segment", "/srv/tenants/*/reports", "/srv/tenants/acme/reports", true},
		{"star no cross slash", "/srv/tenants/*/reports", "/srv/tenants/acme/private/reports", false},
		{"star three deep", "/srv/tenants/*/reports", "/srv/tenants/acme/private/deeper/reports", false},
		{"star trailing empty", "/srv/tenants/*", "/srv/tenants/", true},
		// "**" spans zero or more segments.
		{"double star deep", "/srv/tenants/acme/**", "/srv/tenants/acme/a/b/c/d", true},
		{"double star zero", "/srv/tenants/acme/**", "/srv/tenants/acme", true},
		// A literal tail covers the directory itself and everything below.
		{"literal tail file", "/srv/tenants/acme/reports", "/srv/tenants/acme/reports/q1.csv", true},
		{"literal tail exact", "/srv/tenants/acme/reports", "/srv/tenants/acme/reports", true},
		{"literal tail sibling", "/srv/tenants/acme/reports", "/srv/tenants/acme/reports-2024", false},
		// Normalised lexical ".." cannot keep the request inside acme.
		{"dot dot escape", "/srv/tenants/acme/**", "/srv/tenants/beta/x", false},
		{"root pattern", "/**", "/srv/tenants/acme/x", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchPath(tc.pattern, tc.target); got != tc.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tc.pattern, tc.target, got, tc.want)
			}
		})
	}
}

func TestValidPatternRejectsUnsupportedWildcards(t *testing.T) {
	for _, p := range []string{
		"/srv/a**/b",
		"/srv/**x",
		"/srv/a?/b",
		"/srv/[abc]/b",
	} {
		if err := ValidPattern(p); err == nil {
			t.Errorf("ValidPattern(%q) = nil, want an error", p)
		}
	}
}
