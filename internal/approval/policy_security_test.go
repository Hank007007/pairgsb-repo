package approval

import "testing"

// TestMatchDoesNotUseStringPrefix reproduces the review finding: an "ops"
// rule must not match "ops-readonly", since the two roles are unrelated.
func TestMatchDoesNotUseStringPrefix(t *testing.T) {
	rules := []Rule{{Role: "ops", Effect: EffectAllow}}
	if got := Match("ops-readonly", rules); got != EffectNone {
		t.Fatalf("Match(ops-readonly) with ops rule = %q, want none", got)
	}
	if Allows("ops-readonly", rules) {
		t.Fatal("Allows(ops-readonly) returned true for an unrelated ops rule")
	}
}

func TestMatchHierarchyIsNotSubstring(t *testing.T) {
	rules := []Rule{{Role: "admin", Effect: EffectAllow}}
	for _, role := range []string{"admin", "ops", "auditor", "ops-oncall", "ops-readonly"} {
		if got := Match(role, rules); got != EffectAllow {
			t.Errorf("Match(%q) with admin rule = %q, want allow", role, got)
		}
	}
}

func TestMatchAncestorChain(t *testing.T) {
	rules := []Rule{{Role: "auditor", Effect: EffectAllow}}
	if got := Match("ops-readonly", rules); got != EffectAllow {
		t.Fatalf("Match(ops-readonly) with auditor rule = %q, want allow", got)
	}
	// ops is not under auditor.
	if got := Match("ops", rules); got != EffectNone {
		t.Fatalf("Match(ops) with auditor rule = %q, want none", got)
	}
}

// TestMatchDenyWinsRegardlessOfOrder reproduces the ordering finding: the
// same role and rule set must give the same answer in either order.
func TestMatchDenyWinsRegardlessOfOrder(t *testing.T) {
	role := "ops-oncall"
	first := []Rule{
		{Role: "admin", Effect: EffectAllow},
		{Role: "ops-oncall", Effect: EffectDeny},
	}
	second := []Rule{
		{Role: "ops-oncall", Effect: EffectDeny},
		{Role: "admin", Effect: EffectAllow},
	}
	if got := Match(role, first); got != EffectDeny {
		t.Fatalf("Match in first order = %q, want deny", got)
	}
	if got := Match(role, second); got != EffectDeny {
		t.Fatalf("Match in reversed order = %q, want deny", got)
	}
}

func TestMatchAllowWhenNoDenyApplies(t *testing.T) {
	role := "ops-oncall"
	rules := []Rule{
		{Role: "admin", Effect: EffectAllow},
		{Role: "ops", Effect: EffectAllow},
	}
	if got := Match(role, rules); got != EffectAllow {
		t.Fatalf("Match(%q) = %q, want allow", role, got)
	}
}

func TestMatchNoApplicableRule(t *testing.T) {
	rules := []Rule{{Role: "ops-oncall", Effect: EffectAllow}}
	if got := Match("ops-readonly", rules); got != EffectNone {
		t.Fatalf("Match(ops-readonly) = %q, want none", got)
	}
}
