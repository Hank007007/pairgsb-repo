package acl_test

import (
	"os"
	"testing"

	"topicwarden/internal/acl"
)

func loadTestRules(t *testing.T) *acl.RuleSet {
	t.Helper()
	data, err := os.ReadFile("../../testdata/rules.yaml")
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rs, err := acl.LoadRules(data)
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	return rs
}

func TestLoadRulesReadsFields(t *testing.T) {
	rs := loadTestRules(t)
	if len(rs.Rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rs.Rules))
	}
	first := rs.Rules[0]
	if first.ID != "sensors-subscribe" || first.Effect != acl.EffectAllow {
		t.Fatalf("unexpected first rule: %+v", first)
	}
	if first.Principal != "sensor" || first.Action != acl.ActionSubscribe {
		t.Fatalf("unexpected first rule: %+v", first)
	}
}

func TestMatchLiteral(t *testing.T) {
	cases := []struct {
		filter, topic string
		want          bool
	}{
		{"sensors/+/temp", "sensors/a/temp", true},
		{"sensors/+/temp", "sensors/a/hum", false},
		{"sensors/a/temp", "sensors/a/temp", true},
		{"sensors/a/temp", "sensors/b/temp", false},
	}
	for _, c := range cases {
		if got := acl.Match(c.filter, c.topic); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.filter, c.topic, got, c.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	rs := loadTestRules(t)

	if v := acl.Evaluate(rs, "sensor", acl.ActionSubscribe, "sensors/a/temp"); !v.Allowed {
		t.Fatalf("expected allow, got %+v", v)
	}
	if v := acl.Evaluate(rs, "sensor", acl.ActionSubscribe, "sensors/a/hum"); v.Allowed {
		t.Fatalf("expected deny, got %+v", v)
	}
	if v := acl.Evaluate(rs, "service", acl.ActionSubscribe, "sensors/a/temp"); v.Allowed {
		t.Fatalf("expected deny for unknown principal, got %+v", v)
	}
}

func TestRuleAppliesToTenant(t *testing.T) {
	open := acl.Rule{ID: "open"}
	if !acl.RuleAppliesToTenant(open, "acme") || !acl.RuleAppliesToTenant(open, "") {
		t.Fatalf("rule without tenant must apply everywhere")
	}

	scoped := acl.Rule{ID: "scoped", Tenant: "acme"}
	if !acl.RuleAppliesToTenant(scoped, "acme") {
		t.Fatalf("rule must apply inside its tenant")
	}
	if acl.RuleAppliesToTenant(scoped, "globex") {
		t.Fatalf("rule must not apply outside its tenant")
	}
}
