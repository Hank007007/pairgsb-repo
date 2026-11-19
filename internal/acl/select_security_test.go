package acl

import (
	"strings"
	"testing"
)

// TestSelectRuleDenyAlwaysWins covers INC-2024-04-19: the child deny must win
// over the parent grant on every call, regardless of map iteration order.
func TestSelectRuleDenyAlwaysWins(t *testing.T) {
	src := `{
		"rules": [
			{"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read", "list"]},
			{"uid": 1001, "paths": ["/srv/tenants/acme/private/**"], "ops": ["deny"]}
		]
	}`

	for i := 0; i < 200; i++ {
		p, err := Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		rule, pattern, ok := p.SelectRule(1001, "/srv/tenants/acme/private/keys.txt")
		if !ok {
			t.Fatalf("iteration %d: no rule selected", i)
		}
		if !rule.Denies() {
			t.Fatalf("iteration %d: selected granting rule via %q, want the deny", i, pattern)
		}
		if pattern != "/srv/tenants/acme/private/**" {
			t.Fatalf("iteration %d: selected pattern %q", i, pattern)
		}
	}
}

func TestSelectRuleLongestLiteralPrefix(t *testing.T) {
	p := parseRules(t, `{
		"rules": [
			{"uid": 1001, "paths": ["/srv/tenants/*/reports"], "ops": ["list"]},
			{"uid": 1001, "paths": ["/srv/tenants/acme/reports/**"], "ops": ["read"]}
		]
	}`)

	rule, pattern, ok := p.SelectRule(1001, "/srv/tenants/acme/reports/q1.csv")
	if !ok {
		t.Fatal("no rule selected")
	}
	if pattern != "/srv/tenants/acme/reports/**" {
		t.Fatalf("selected %q, want the longer literal prefix", pattern)
	}
	if !rule.Allows(OpRead) {
		t.Fatal("selected rule does not grant read")
	}
}

func TestSelectRuleNoMatchForUnknownTenant(t *testing.T) {
	p := parseRules(t, `{
		"rules": [
			{"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read"]}
		]
	}`)

	for _, target := range []string{
		"/srv/tenants/acme-evil/x",
		"/srv/tenants/acmex",
		"/srv/packages/agent-v3.tar",
	} {
		if _, _, ok := p.SelectRule(1001, target); ok {
			t.Errorf("SelectRule matched %q against the acme rule", target)
		}
	}
}

func TestTenantRoot(t *testing.T) {
	p := parseRules(t, `{
		"rules": [
			{"uid": 1001, "paths": ["/srv/tenants/acme/**"], "ops": ["read", "chown"]},
			{"uid": 1001, "paths": ["/srv/tenants/acme/private/**"], "ops": ["deny"]},
			{"uid": 1002, "paths": ["/srv/tenants/beta/**"], "ops": ["read"]}
		]
	}`)

	root, ok := p.TenantRoot(1001, OpChown)
	if !ok || root != "/srv/tenants/acme" {
		t.Fatalf("tenant root = %q, ok=%v", root, ok)
	}
	if _, ok := p.TenantRoot(1002, OpChown); ok {
		t.Fatal("uid 1002 has no chown rule but got a tenant root")
	}
}
