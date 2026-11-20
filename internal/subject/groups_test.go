package subject

import "testing"

const sampleGroups = `
users:
  alice:
    - platform
  bob:
    - db-oncall
  carol: []
groups:
  platform:
    - sre
  sre:
    - db-oncall
  db-oncall: []
`

func TestParseDirectoryUsersAndGroups(t *testing.T) {
	dir, err := ParseDirectory(sampleGroups)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	if got := dir.Users["alice"]; len(got) != 1 || got[0] != "platform" {
		t.Fatalf("alice groups = %v, want [platform]", got)
	}
	if got := dir.Groups["sre"]; len(got) != 1 || got[0] != "db-oncall" {
		t.Fatalf("sre children = %v, want [db-oncall]", got)
	}
}

func TestExpandIncludesDirectGroups(t *testing.T) {
	dir, err := ParseDirectory(sampleGroups)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	groups := dir.Expand("carol", []string{"auditors"})
	if !contains(groups, "auditors") {
		t.Fatalf("expanded groups = %v, want to contain auditors", groups)
	}
}

func TestExpandIncludesUserMembership(t *testing.T) {
	dir, err := ParseDirectory(sampleGroups)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	groups := dir.Expand("bob", nil)
	if !contains(groups, "db-oncall") {
		t.Fatalf("expanded groups = %v, want to contain db-oncall", groups)
	}
}

func TestExpandDeduplicates(t *testing.T) {
	dir, err := ParseDirectory(sampleGroups)
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}
	groups := dir.Expand("alice", []string{"platform"})
	seen := map[string]int{}
	for _, group := range groups {
		seen[group]++
	}
	for group, count := range seen {
		if count != 1 {
			t.Fatalf("group %q appears %d times in %v", group, count, groups)
		}
	}
}

func TestParseDirectoryRejectsUnknownSection(t *testing.T) {
	if _, err := ParseDirectory("members:\n  alice:\n    - sre\n"); err == nil {
		t.Fatalf("expected error for unknown section")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
