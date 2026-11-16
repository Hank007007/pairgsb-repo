package policy

import "testing"

func testSubject(roles ...string) Subject {
	return Subject{
		ServiceAccount: "svc-test",
		Roles:          roles,
		RoleVersion:    "1",
	}
}

func TestEvaluateSingleAllowRule(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{
				Name:     "reader",
				Roles:    []string{"reader"},
				Resource: "/orgs/acme/**",
				Actions:  []string{"read"},
				Effect:   Allow,
			},
		},
	}
	engine := NewEngine(p)

	decision := engine.Evaluate(Request{
		Subject:  testSubject("reader"),
		Resource: "/orgs/acme/projects/42",
		Action:   "read",
	})
	if !decision.Allowed() {
		t.Fatalf("expected allow, got %+v", decision)
	}
	if decision.Source != SourcePolicy {
		t.Fatalf("expected source %q, got %q", SourcePolicy, decision.Source)
	}
}

func TestEvaluateRoleWithoutGrant(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{
				Name:     "reader",
				Roles:    []string{"reader"},
				Resource: "/orgs/acme/**",
				Actions:  []string{"read"},
				Effect:   Allow,
			},
		},
	}
	engine := NewEngine(p)

	decision := engine.Evaluate(Request{
		Subject:  testSubject("writer"),
		Resource: "/orgs/acme/projects/42",
		Action:   "read",
	})
	if decision.Allowed() {
		t.Fatalf("expected deny, got %+v", decision)
	}
}

func TestEvaluateNoMatchingRule(t *testing.T) {
	engine := NewEngine(&Policy{})

	decision := engine.Evaluate(Request{
		Subject:  testSubject("reader"),
		Resource: "/orgs/acme/projects/42",
		Action:   "read",
	})
	if decision.Allowed() {
		t.Fatalf("expected deny, got %+v", decision)
	}
}

func TestEvaluateDelegatedGrant(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{
				Name:     "owner",
				Roles:    []string{"org-owner"},
				Resource: "/orgs/acme/**",
				Actions:  []string{"read", "write"},
				Effect:   Allow,
			},
		},
		Delegations: []Delegation{
			{From: "org-owner", To: "auditor", Resource: "/orgs/acme/reports/**", Actions: []string{"read"}},
		},
	}
	engine := NewEngine(p)

	decision := engine.Evaluate(Request{
		Subject:  testSubject("auditor"),
		Resource: "/orgs/acme/reports/2026",
		Action:   "read",
	})
	if !decision.Allowed() {
		t.Fatalf("expected inherited allow, got %+v", decision)
	}
}
