package policy

import "testing"

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"www.example.com", "www.example.com"},
		{"WWW.Example.COM", "www.example.com"},
		{"www.example.com.", "www.example.com"},
		{"WWW.Example.COM.", "www.example.com"},
	}
	for _, tc := range cases {
		if got := normalizeHost(tc.in); got != tc.want {
			t.Errorf("normalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
