package policy_test

import (
	"testing"

	"maskseal-journal/internal/policy"
)

func TestParseRules(t *testing.T) {
	rs, err := policy.Parse([]byte(`{
		"rules": [
			{"path": "password", "action": "redact"},
			{"path": "role", "action": "keep"}
		]
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rs.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(rs.Rules))
	}
	if rs.Rules[0].Action != policy.ActionRedact || rs.Rules[1].Action != policy.ActionKeep {
		t.Fatalf("unexpected actions: %+v", rs.Rules)
	}
}

func TestParseDefaultsActionToRedact(t *testing.T) {
	rs, err := policy.Parse([]byte(`{"rules":[{"path":"password"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rs.Rules[0].Action != policy.ActionRedact {
		t.Fatalf("action = %q, want redact", rs.Rules[0].Action)
	}
}

func TestMatchExactScalarPath(t *testing.T) {
	rs, err := policy.Parse([]byte(`{"rules":[{"path":"password","action":"redact"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	action, matched := rs.Match("password")
	if !matched {
		t.Fatal("expected a match for password")
	}
	if action != policy.ActionRedact {
		t.Fatalf("action = %q, want redact", action)
	}
}

func TestFirstMatchingRuleWins(t *testing.T) {
	rs, err := policy.Parse([]byte(`{"rules":[
		{"path":"password","action":"keep"},
		{"path":"password","action":"redact"}
	]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	action, _ := rs.Match("password")
	if action != policy.ActionKeep {
		t.Fatalf("action = %q, want keep", action)
	}
}

func TestEmptyPolicyIsUsable(t *testing.T) {
	rs, err := policy.Parse([]byte(`{"rules":[]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !rs.Empty() {
		t.Fatal("expected an empty rule set")
	}
}
