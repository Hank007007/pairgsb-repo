package subject

import "testing"

const nestedGroups = `
users:
  alice:
    - platform
groups:
  platform:
    - sre
  sre:
    - db-oncall
  db-oncall:
    - runtime
  runtime: []
`

func TestExpandRecursesToFixedPoint(t *testing.T) {
	dir, err := ParseDirectory(nestedGroups)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	groups := dir.Expand("alice", nil)
	for _, want := range []string{"platform", "sre", "db-oncall", "runtime"} {
		if !contains(groups, want) {
			t.Fatalf("expanded groups = %v, want to contain %q", groups, want)
		}
	}
}

func TestExpandAlwaysIncludesAuthenticatedGroup(t *testing.T) {
	dir, err := ParseDirectory(nestedGroups)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	for _, tc := range []struct {
		user   string
		groups []string
	}{
		{"alice", nil},
		{"unknown-user", nil},
		{"unknown-user", []string{"auditors"}},
	} {
		groups := dir.Expand(tc.user, tc.groups)
		if !contains(groups, SystemAuthenticated) {
			t.Fatalf("groups for %v = %v, missing %q", tc, groups, SystemAuthenticated)
		}
	}
}

func TestExpandTerminatesOnCyclesAndDeduplicates(t *testing.T) {
	dir, err := ParseDirectory(`
users:
  alice:
    - a
groups:
  a:
    - b
  b:
    - a
`)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	groups := dir.Expand("alice", []string{"a", "a"})
	counts := map[string]int{}
	for _, group := range groups {
		counts[group]++
	}
	for group, count := range counts {
		if count != 1 {
			t.Fatalf("group %q appears %d times in %v", group, count, groups)
		}
	}
	for _, want := range []string{"a", "b", SystemAuthenticated} {
		if !contains(groups, want) {
			t.Fatalf("groups = %v, want to contain %q", groups, want)
		}
	}
}
