package approval

import "testing"

func TestMatchExactRole(t *testing.T) {
	policy := []Rule{
		{Role: "admin", Effect: EffectAllow},
		{Role: "ops", Effect: EffectAllow},
	}
	if got := Match("ops", policy); got != EffectAllow {
		t.Fatalf("Match(ops) = %q", got)
	}
	if got := Match("admin", policy); got != EffectAllow {
		t.Fatalf("Match(admin) = %q", got)
	}
}

func TestMatchFallsBackToAncestorRule(t *testing.T) {
	policy := []Rule{
		{Role: "ops", Effect: EffectAllow},
	}
	if got := Match("ops-oncall", policy); got != EffectAllow {
		t.Fatalf("Match(ops-oncall) = %q", got)
	}
}

func TestMatchWithoutRule(t *testing.T) {
	policy := []Rule{
		{Role: "ops", Effect: EffectAllow},
	}
	if got := Match("auditor", policy); got != EffectNone {
		t.Fatalf("Match(auditor) = %q", got)
	}
	if Allows("auditor", policy) {
		t.Fatal("Allows(auditor) returned true without a matching rule")
	}
}

func TestDenyRuleApplies(t *testing.T) {
	policy := []Rule{
		{Role: "auditor", Effect: EffectDeny},
	}
	if got := Match("auditor", policy); got != EffectDeny {
		t.Fatalf("Match(auditor) = %q", got)
	}
	if Allows("auditor", policy) {
		t.Fatal("Allows(auditor) returned true for a deny rule")
	}
}

func TestAllowsWithSpecificRole(t *testing.T) {
	policy := []Rule{
		{Role: "admin", Effect: EffectAllow},
		{Role: "ops-oncall", Effect: EffectAllow},
	}
	if !Allows("ops-oncall", policy) {
		t.Fatal("Allows(ops-oncall) returned false")
	}
}
