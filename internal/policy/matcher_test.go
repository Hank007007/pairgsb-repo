package policy

import (
	"testing"
)

func TestMatchGlobSingleSegment(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"kv/app/*", "kv/app/db", true},
		{"kv/app/*", "kv/other/db", false},
		{"kv/app/payments/*", "kv/app/payments/stripe", true},
		{"kv/app/payments/*", "kv/app/payments", false},
		{"kv/*/stripe", "kv/app/stripe", true},
		{"kv/app/payments/stripe", "kv/app/payments/stripe", true},
		{"kv/app/payments/stripe", "kv/app/payments/refund", false},
	}
	for _, tc := range cases {
		if got := MatchGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestMatchGlobDoubleStar(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"kv/app/**", "kv/app", true},
		{"kv/app/**", "kv/app/db", true},
		{"kv/app/**", "kv/app/db/prod", true},
		{"kv/app/**", "kv/other/db", false},
		{"kv/**/password", "kv/app/db/prod/password", true},
	}
	for _, tc := range cases {
		if got := MatchGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestEvaluateAllowOnly(t *testing.T) {
	acc := &Accessor{
		ID:    "svc-payments",
		Token: "acc_payments",
		Rules: []Rule{{Effect: Allow, Path: "kv/app/payments/*"}},
	}
	if !acc.Evaluate("kv/app/payments/stripe") {
		t.Error("expected matching path to be allowed")
	}
	if acc.Evaluate("kv/app/db/password") {
		t.Error("expected unrelated path to be denied")
	}
}

func TestEvaluateDenyOnly(t *testing.T) {
	acc := &Accessor{
		ID:    "svc-blocked",
		Token: "acc_blocked",
		Rules: []Rule{{Effect: Deny, Path: "kv/app/payments/**"}},
	}
	if acc.Evaluate("kv/app/payments/stripe") {
		t.Error("expected path covered by a deny rule to be refused")
	}
	// With no matching allow rule the outcome is still a refusal.
	if acc.Evaluate("kv/app/db/password") {
		t.Error("expected unmatched path to be refused")
	}
}

func TestEvaluateDisjointRules(t *testing.T) {
	acc := &Accessor{
		ID:    "svc-mixed",
		Token: "acc_mixed",
		Rules: []Rule{
			{Effect: Allow, Path: "kv/app/payments/*"},
			{Effect: Deny, Path: "kv/internal/**"},
		},
	}
	if !acc.Evaluate("kv/app/payments/stripe") {
		t.Error("expected allow rule to cover payments path")
	}
	if acc.Evaluate("kv/internal/root") {
		t.Error("expected deny rule to cover internal path")
	}
}

func TestPolicyLookup(t *testing.T) {
	p, err := New([]Accessor{{
		ID:    "svc-a",
		Token: "tok-a",
		Rules: []Rule{{Effect: Allow, Path: "kv/**"}},
	}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	acc, ok := p.ByToken("tok-a")
	if !ok || acc.ID != "svc-a" {
		t.Fatalf("ByToken(tok-a) = %+v, %v", acc, ok)
	}
	if _, ok := p.ByToken("missing"); ok {
		t.Error("expected unknown token to be absent")
	}
}

func TestPolicyRejectsDuplicateTokens(t *testing.T) {
	_, err := New([]Accessor{
		{ID: "a", Token: "same"},
		{ID: "b", Token: "same"},
	})
	if err == nil {
		t.Fatal("expected duplicate token to be rejected")
	}
}

func TestLoadTestdataPolicies(t *testing.T) {
	for _, file := range []string{
		"../../testdata/policy-payments.json",
		"../../testdata/policy-ops.json",
	} {
		accessors, err := Load(file)
		if err != nil {
			t.Fatalf("Load(%s): %v", file, err)
		}
		if len(accessors) == 0 {
			t.Fatalf("Load(%s) returned no accessors", file)
		}
	}
}
