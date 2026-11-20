package authz

import "testing"

func allowRule(verb, group, resource string) Rule {
	return Rule{Verbs: []string{verb}, APIGroups: []string{group}, Resources: []string{resource}}
}

func TestEvaluateAllowsMatchingUserBinding(t *testing.T) {
	bindings := []Binding{
		{
			Namespace: "data",
			Subjects:  []Subject{{Kind: "User", Name: "alice"}},
			Rules:     []Rule{allowRule("get", "", "pods")},
		},
	}
	result := Evaluate(
		Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"},
		Identity{User: "alice"},
		bindings,
	)
	if result.Decision != DecisionAllow {
		t.Fatalf("decision = %s, want allow", result.Decision)
	}
}

func TestEvaluateAllowsMatchingGroupBinding(t *testing.T) {
	bindings := []Binding{
		{
			Namespace: "platform",
			Subjects:  []Subject{{Kind: "Group", Name: "sre"}},
			Rules:     []Rule{allowRule("list", "", "pods")},
		},
	}
	result := Evaluate(
		Request{Verb: "list", APIGroup: "", Resource: "pods", Namespace: "platform"},
		Identity{User: "alice", Groups: []string{"sre"}},
		bindings,
	)
	if result.Decision != DecisionAllow {
		t.Fatalf("decision = %s, want allow", result.Decision)
	}
}

func TestEvaluateNoOpinionWithoutMatch(t *testing.T) {
	bindings := []Binding{
		{
			Namespace: "data",
			Subjects:  []Subject{{Kind: "User", Name: "alice"}},
			Rules:     []Rule{allowRule("get", "", "pods")},
		},
	}
	result := Evaluate(
		Request{Verb: "delete", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"},
		Identity{User: "alice"},
		bindings,
	)
	if result.Decision != DecisionNoOpinion {
		t.Fatalf("decision = %s, want no-opinion", result.Decision)
	}
}

func TestEvaluateIgnoresBindingInOtherNamespace(t *testing.T) {
	bindings := []Binding{
		{
			Namespace: "staging",
			Subjects:  []Subject{{Kind: "User", Name: "alice"}},
			Rules:     []Rule{allowRule("get", "", "pods")},
		},
	}
	result := Evaluate(
		Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "production", Name: "db-prod"},
		Identity{User: "alice"},
		bindings,
	)
	if result.Decision != DecisionNoOpinion {
		t.Fatalf("decision = %s, want no-opinion", result.Decision)
	}
}

func TestEvaluateClusterBindingAppliesEverywhere(t *testing.T) {
	bindings := []Binding{
		{
			Subjects: []Subject{{Kind: "Group", Name: "system:authenticated"}},
			Rules:    []Rule{allowRule("get", "", "namespaces")},
		},
	}
	result := Evaluate(
		Request{Verb: "get", APIGroup: "", Resource: "namespaces", Name: "default"},
		Identity{User: "bob", Groups: []string{"system:authenticated"}},
		bindings,
	)
	if result.Decision != DecisionAllow {
		t.Fatalf("decision = %s, want allow", result.Decision)
	}
}

func TestEvaluateRuleWithoutEffectDefaultsToAllow(t *testing.T) {
	rule := allowRule("get", "", "pods")
	rule.Effect = ""
	bindings := []Binding{
		{Namespace: "data", Subjects: []Subject{{Kind: "User", Name: "alice"}}, Rules: []Rule{rule}},
	}
	result := Evaluate(
		Request{Verb: "get", APIGroup: "", Resource: "pods", Namespace: "data", Name: "db-prod"},
		Identity{User: "alice"},
		bindings,
	)
	if result.Decision != DecisionAllow {
		t.Fatalf("decision = %s, want allow", result.Decision)
	}
}
