package policy

import "testing"

func TestMatchResourceSuffixWildcard(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/orgs/acme/**", "/orgs/acme", true},
		{"/orgs/acme/**", "/orgs/acme/projects/42", true},
		{"/**", "/anything/at/all", true},
		{"/orgs/acme/**", "/orgs/other/projects", false},
	}
	for _, tc := range cases {
		if got := MatchResource(tc.pattern, tc.path); got != tc.want {
			t.Errorf("MatchResource(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"/orgs/acme":        "/orgs/acme",
		"/orgs//acme/":      "/orgs/acme",
		"/orgs/acme/":       "/orgs/acme",
		"/":                 "/",
		"":                  "/",
		"/orgs//acme//x///": "/orgs/acme/x",
	}
	for in, want := range cases {
		if got := NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}
