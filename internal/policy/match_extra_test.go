package policy

import (
	"strconv"
	"testing"
)

func TestAllowRequiresExactSegmentMatch(t *testing.T) {
	rule := Rule{ID: "r", From: "payments", To: "/ledger.v1.Ledger/Debit", Effect: EffectPermit}

	for _, peer := range []string{"payments-backup", "payments2", "paymentsx", "payments/internal"} {
		if Allow(peer, "/ledger.v1.Ledger/Debit", rule) {
			t.Errorf("Allow(%q) = true, want false", peer)
		}
	}
	if !Allow("payments", "/ledger.v1.Ledger/Debit", rule) {
		t.Error("Allow(payments) = false, want true")
	}
}

func TestAllowWildcardMatchesExactlyOneSegment(t *testing.T) {
	rule := Rule{ID: "r", From: "*", To: "/ledger.v1.Ledger/Balance", Effect: EffectPermit}

	if !Allow("payments", "/ledger.v1.Ledger/Balance", rule) {
		t.Error("* should match single-segment identity payments")
	}
	if Allow("payments/v2", "/ledger.v1.Ledger/Balance", rule) {
		t.Error("* must not match a multi-segment identity")
	}
	if Allow("", "/ledger.v1.Ledger/Balance", rule) {
		t.Error("* must not match an empty identity")
	}
}

func TestAllowTwoSegmentWildcard(t *testing.T) {
	rule := Rule{ID: "r", From: "payments/*", To: "/m/M", Effect: EffectPermit}

	if !Allow("payments/v2", "/m/M", rule) {
		t.Error("payments/* should match payments/v2")
	}
	if Allow("payments", "/m/M", rule) {
		t.Error("payments/* must not match payments itself")
	}
	if Allow("payments/v2/eu", "/m/M", rule) {
		t.Error("payments/* must not match three segments")
	}
}

func TestCheckDenyAlwaysWinsAndIsStable(t *testing.T) {
	rules := []Rule{
		{ID: "permit", From: "reporting", To: "/ledger.v1.Ledger/*", Effect: EffectPermit},
		{ID: "deny", From: "reporting", To: "/ledger.v1.Ledger/Debit", Effect: EffectDeny},
	}

	// Repeat across many checks to catch any dependence on map iteration order.
	for i := 0; i < 200; i++ {
		set := &PolicySet{Rules: map[string]Rule{
			"permit-" + strconv.Itoa(i): rules[0],
			"deny-" + strconv.Itoa(i):   rules[1],
		}}
		if err := set.Check("reporting", "/ledger.v1.Ledger/Debit"); err == nil {
			t.Fatalf("iteration %d: deny rule was overridden by permit", i)
		}
		if err := set.Check("reporting", "/ledger.v1.Ledger/Balance"); err != nil {
			t.Fatalf("iteration %d: permitted method was denied: %v", i, err)
		}
	}
}

func TestCheckDeniesWhenNoRuleMatches(t *testing.T) {
	set := &PolicySet{Rules: map[string]Rule{
		"r": {ID: "r", From: "payments", To: "/ledger.v1.Ledger/Debit", Effect: EffectPermit},
	}}

	if err := set.Check("stranger", "/ledger.v1.Ledger/Debit"); err == nil {
		t.Fatal("Check allowed a peer with no matching rule")
	}
	if err := set.Check("", "/ledger.v1.Ledger/Debit"); err == nil {
		t.Fatal("Check allowed an empty identity")
	}
}
