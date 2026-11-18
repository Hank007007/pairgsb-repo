package acl

import (
	"testing"
)

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/srv/tenants/acme", "/srv/tenants/acme"},
		{"/srv//tenants///acme/", "/srv/tenants/acme"},
		{"/srv/tenants/acme/./reports", "/srv/tenants/acme/reports"},
		{"/srv/tenants/acme/../beta", "/srv/tenants/beta"},
		{"/", "/"},
	}

	for _, tc := range cases {
		got, err := NormalizePath(tc.in)
		if err != nil {
			t.Fatalf("NormalizePath(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizePathRejectsRelative(t *testing.T) {
	for _, in := range []string{"", "srv/tenants/acme", "./acme", "../acme"} {
		if _, err := NormalizePath(in); err == nil {
			t.Errorf("NormalizePath(%q) accepted a relative path", in)
		}
	}
}

func TestValidPattern(t *testing.T) {
	valid := []string{
		"/srv/tenants/acme/**",
		"/srv/tenants/*/reports",
		"/srv/tenants/acme",
		"/",
	}
	for _, p := range valid {
		if err := ValidPattern(p); err != nil {
			t.Errorf("ValidPattern(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{
		"",
		"srv/tenants/acme/**",
		"/srv/tenants/../acme/**",
		"/srv/./acme",
	}
	for _, p := range invalid {
		if err := ValidPattern(p); err == nil {
			t.Errorf("ValidPattern(%q) = nil, want an error", p)
		}
	}
}

func TestPatternRoot(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/srv/tenants/acme/**", "/srv/tenants/acme"},
		{"/srv/tenants/acme", "/srv/tenants/acme"},
		{"/srv/tenants/*/reports", "/srv/tenants"},
		{"/srv/**", "/srv"},
	}

	for _, tc := range cases {
		if got := PatternRoot(tc.in); got != tc.want {
			t.Errorf("PatternRoot(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMatchPathPrefix(t *testing.T) {
	cases := []struct {
		pattern string
		target  string
		want    bool
	}{
		{"/srv/tenants/acme/**", "/srv/tenants/acme/reports/q1.csv", true},
		{"/srv/tenants/acme/**", "/srv/tenants/acme", true},
		{"/srv/tenants/acme/**", "/srv/tenants/beta/reports/q1.csv", false},
		{"/srv/tenants/acme", "/srv/tenants/acme/reports", true},
		{"/srv/tenants/acme", "/srv/tenants/beta", false},
	}

	for _, tc := range cases {
		if got := matchPath(tc.pattern, tc.target); got != tc.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tc.pattern, tc.target, got, tc.want)
		}
	}
}
