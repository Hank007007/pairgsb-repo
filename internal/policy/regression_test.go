package policy

import (
	"testing"

	"condgrate/internal/cond"
)

func TestMatchesAttrSegmentWildcards(t *testing.T) {
	cases := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"*", "document", true},
		{"tenant:*", "tenant:eu", true},
		{"tenant:*", "tenant:eu:finance", false},
		{"tenant:*:*", "tenant:eu:finance", true},
		{"super-admin", "admin", false},
	}
	for _, tc := range cases {
		if got := MatchesAttr([]string{tc.pattern}, tc.value); got != tc.want {
			t.Errorf("MatchesAttr(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
		}
	}
}

func TestCollectWalksEveryLayer(t *testing.T) {
	chain := &Chain{
		Org: Scope{
			Layer: OrgLayer,
			Target: Target{
				Principal: MatchBlock{"role": {"*"}},
				Action:    []string{"read"},
				Resource:  MatchBlock{"type": {"*"}},
			},
			Policies: []Policy{{
				ID:        "org-deny-contractors",
				Effect:    Deny,
				Principal: MatchBlock{"role": {"contractor"}},
				Action:    []string{"read"},
				Resource:  MatchBlock{"type": {"*"}},
			}},
		},
		Project: Scope{
			Layer: ProjectLayer,
			Target: Target{
				Principal: MatchBlock{"role": {"*"}},
				Action:    []string{"read"},
				Resource:  MatchBlock{"type": {"*"}},
			},
			Policies: []Policy{{
				ID:          "project-read",
				Effect:      Permit,
				Principal:   MatchBlock{"role": {"contractor"}},
				Action:      []string{"read"},
				Resource:    MatchBlock{"type": {"document"}},
				Obligations: []string{"audit-log"},
			}},
		},
	}
	req := MatchRequest{
		Principal: MatchBlock{"role": {"contractor"}},
		Resource:  MatchBlock{"type": {"document"}},
		Action:    "read",
	}

	candidates := Collect(chain, req, testEnv{})
	if len(candidates) != 2 {
		t.Fatalf("candidates = %v, want project permit and org deny", candidates)
	}
	decision := Combine(candidates)
	if decision.Effect != Deny {
		t.Fatalf("effect = %v, want deny", decision.Effect)
	}
	if len(decision.Obligations) != 0 {
		t.Fatalf("obligations = %v, want none on deny", decision.Obligations)
	}
}

func TestCombineDenyWinsInEitherOrder(t *testing.T) {
	permit := Candidate{
		Policy:  Policy{ID: "permit", Effect: Permit, Obligations: []string{"audit-log"}},
		Verdict: cond.True,
	}
	deny := Candidate{
		Policy:  Policy{ID: "deny", Effect: Deny},
		Verdict: cond.True,
	}

	for _, candidates := range [][]Candidate{{permit, deny}, {deny, permit}} {
		got := Combine(candidates)
		if got.Effect != Deny {
			t.Errorf("effect = %v, want deny", got.Effect)
		}
		if len(got.Obligations) != 0 {
			t.Errorf("obligations = %v, want none", got.Obligations)
		}
	}
}

func TestCombineObligationsOnlyFromTruePermitPolicies(t *testing.T) {
	got := Combine([]Candidate{
		{
			Policy:  Policy{ID: "permit", Effect: Permit, Obligations: []string{"audit-log"}},
			Verdict: cond.True,
		},
		{
			Policy:  Policy{ID: "false-permit", Effect: Permit, Obligations: []string{"notify-owner"}},
			Verdict: cond.False,
		},
	})
	if got.Effect != Permit {
		t.Fatalf("effect = %v, want permit", got.Effect)
	}
	if len(got.Obligations) != 1 || got.Obligations[0] != "audit-log" {
		t.Fatalf("obligations = %v, want only audit-log", got.Obligations)
	}
}

func TestCombineIndeterminateFailsClosed(t *testing.T) {
	got := Combine([]Candidate{
		{
			Policy:  Policy{ID: "permit", Effect: Permit, Obligations: []string{"audit-log"}},
			Verdict: cond.True,
		},
		{
			Policy:  Policy{ID: "unknown", Effect: Permit, Obligations: []string{"notify-owner"}},
			Verdict: cond.Indeterminate,
		},
	})
	if got.Effect != Deny {
		t.Fatalf("effect = %v, want deny", got.Effect)
	}
	if len(got.Obligations) != 0 {
		t.Fatalf("obligations = %v, want none", got.Obligations)
	}
}
