package acl

import (
	"strings"
	"testing"
)

func parseRules(t *testing.T, src string) *Policy {
	t.Helper()

	p, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestParseAccessFile(t *testing.T) {
	p := parseRules(t, `{
		"rules": [
			{"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read", "list"]},
			{"uid": 1001, "paths": ["/srv/tenants/acme/private/**"], "ops": ["deny"]},
			{"uid": 1002, "paths": ["/srv/tenants/beta"], "ops": ["read", "write", "chown"]}
		]
	}`)

	if got := len(p.Rules(1001)); got != 2 {
		t.Fatalf("uid 1001 has %d rules, want 2", got)
	}
	if got := len(p.Rules(1002)); got != 1 {
		t.Fatalf("uid 1002 has %d rules, want 1", got)
	}
	if got := len(p.Rules(1003)); got != 0 {
		t.Fatalf("uid 1003 has %d rules, want 0", got)
	}
}

func TestParseRejectsBadRules(t *testing.T) {
	cases := map[string]string{
		"unknown op":        `{"rules": [{"uid": 1001, "paths": ["/srv/acme"], "ops": ["execute"]}]}`,
		"relative path":     `{"rules": [{"uid": 1001, "paths": ["srv/acme"], "ops": ["read"]}]}`,
		"dotdot in path":    `{"rules": [{"uid": 1001, "paths": ["/srv/../acme"], "ops": ["read"]}]}`,
		"no paths":          `{"rules": [{"uid": 1001, "paths": [], "ops": ["read"]}]}`,
		"no ops":            `{"rules": [{"uid": 1001, "paths": ["/srv/acme"], "ops": []}]}`,
		"negative uid":      `{"rules": [{"uid": -1, "paths": ["/srv/acme"], "ops": ["read"]}]}`,
		"deny mixed":        `{"rules": [{"uid": 1001, "paths": ["/srv/acme"], "ops": ["deny", "read"]}]}`,
		"unknown top-level": `{"rules": [], "extra": true}`,
	}

	for name, src := range cases {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("%s: Parse accepted an invalid document", name)
		}
	}
}

func TestSelectRuleSingleMatch(t *testing.T) {
	p := parseRules(t, `{
		"rules": [
			{"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read", "list"]},
			{"uid": 1002, "paths": ["/srv/tenants/beta/**"], "ops": ["read"]}
		]
	}`)

	r, pattern, ok := p.SelectRule(1001, "/srv/tenants/acme/reports/q1.csv")
	if !ok {
		t.Fatal("SelectRule found no rule for a covered path")
	}
	if pattern != "/srv/tenants/acme/**" {
		t.Fatalf("SelectRule selected pattern %q", pattern)
	}
	if !r.Allows(OpRead) {
		t.Fatal("SelectRule returned a rule that does not grant read")
	}

	if _, _, ok := p.SelectRule(1003, "/srv/tenants/acme/reports/q1.csv"); ok {
		t.Fatal("SelectRule matched a uid with no rules")
	}
	if _, _, ok := p.SelectRule(1001, "/srv/tenants/gamma/x"); ok {
		t.Fatal("SelectRule matched a path outside every rule")
	}
}

func TestUIDs(t *testing.T) {
	p := parseRules(t, `{
		"rules": [
			{"uid": 1002, "paths": ["/srv/beta"], "ops": ["read"]},
			{"uid": 1001, "paths": ["/srv/acme"], "ops": ["read"]}
		]
	}`)

	uids := p.UIDs()
	if len(uids) != 2 || uids[0] != 1001 || uids[1] != 1002 {
		t.Fatalf("UIDs() = %v, want [1001 1002]", uids)
	}
}
